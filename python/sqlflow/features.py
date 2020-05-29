# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import ast
import importlib
import inspect
import re


def get_feature_metas(conn, select, label=None):
    cursor = conn.cursor()
    cursor.execute(select)
    feature_metas = {}
    feature_column_names = []
    rows = list(cursor.fetchmany(1024))
    for i, column in enumerate(cursor.description):
        name, typ = column[0], column[1]
        sep, shape, maxid, vocab = "", [1], 0, set()
        if typ in (4, 5):
            dtype = "float32"
        elif typ in (1, 3, 8):
            dtype = "int64"
            vocab.update(r[i] for r in rows)
        elif typ in (252, 253):
            dtype = "string"
            vocab.update(r[i] for r in rows)
            possible_sep = set()
            for row in rows:
                possible_sep.update(re.sub('[0-9]', '', row[i]))
            if len(possible_sep) in (1, 2):
                if len(possible_sep) == 2 and '.' in possible_sep:
                    dtype = "float32"
                    possible_sep.remove('.')
                    sep = possible_sep.pop()
                elif len(possible_sep) == 1:
                    sep = possible_sep.pop()
                    dtype = "int64"
                    for r in rows:
                        maxid = max(maxid,
                                    max(int(p) for p in r[i].split(sep)))
                shape = [len(row[i].split(sep))]
        else:
            raise Exception(f"Unexpected column type {typ}: {name}")
        feature_metas[name] = {
            "feature_name": name,
            "dtype": dtype,
            "delimiter": sep,
            "shape": shape,
            "is_sparse": False,
            "maxid": maxid,
            "vocab": list(vocab)
        }
    label_meta = {} if not label else feature_metas.pop(label)
    if label_meta["shape"] == [1]:
        label_meta["shape"] = []
    return feature_metas, label_meta


def check_and_expand_columns(c, funcs, fields):
    t = ast.parse(c, mode='eval')
    body, leaf = t.body, t.body.args[0]
    while isinstance(leaf, ast.Call):
        body, leaf = leaf, leaf.args[0]
    ret = []
    if isinstance(leaf, ast.Str):
        for f in fields:
            if re.match(leaf.s, f):
                body.args[0] = ast.Name(f,
                                        ast.Load(),
                                        lineno=1,
                                        col_offset=leaf.col_offset)
                ret.append((f, compile(t, '', 'eval')))
        return ret
    return [(leaf.id, compile(t, '', 'eval'))]


def get_column_module(name):
    return importlib.import_module(f"sqlflow.column.{name}")


def get_feature_columns(feature_metas, columns, engine='tensorflow'):
    fc = get_column_module(engine)
    eval_globals = dict(
        filter(lambda x: inspect.isfunction(x[1]),
               vars(fc).items()))
    eval_globals.update(fc.EVAL_GLOBALS)
    ret = {k: [] for k in columns} if columns else {"feature_columns": []}
    # undefined = []
    for k, v in columns.items(
    ):  # k is a parameter name like 'dnn_feature_column'
        for c in v.columns:  # v(ir_pb2.Columns) is a list of expressions
            if c not in feature_metas:  # COLUMN sepal_length
                for i, code_obj in check_and_expand_columns(
                        c.lower(), eval_globals, feature_metas):
                    ret[k].append(eval(code_obj, eval_globals, feature_metas))
    if len(ret) == 1:
        ret = {"feature_columns": list(ret.values())[0]}
    specified_fields = set()
    for k, v in ret.items():
        specified_fields.update(map(lambda f: f.key, v))
    features = ret[list(ret.keys())[0]]
    for k in set(feature_metas.keys()) - specified_fields:
        if feature_metas[k]["dtype"] in ("float32", "int64"):
            features.append(fc.numeric(feature_metas[k]))
    return ret


def get_xgb_features(feature_metas, columns):
    if not columns:
        return None
    from sqlflow_submitter.xgboost import feature_column as xfc
    fc = get_feature_columns(feature_metas, columns, 'xgboost')
    return xfc.ComposedColumnTransformer(list(feature_metas.keys()),
                                         *fc["feature_columns"])


def get_tf_features(feature_metas, columns):
    return get_feature_columns(feature_metas, columns, 'tensorflow')
