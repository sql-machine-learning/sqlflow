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

import base64

import numpy as np
import pandas as pd
import runtime.temp_file as temp_file
import runtime.xgboost as xgboost_extended
import scipy
import shap
import six
import xgboost as xgb
from runtime import db, explainer
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model.model import Model


def infer_data_type(feature):
    if isinstance(feature, np.ndarray):
        if feature.dtype == np.float32 or feature.dtype == np.float64:
            return 'float32'
        elif feature.dtype == np.int32 or feature.dtype == np.int64:
            return 'int64'
        else:
            raise ValueError('Not supported data type {}'.format(
                feature.dtype))
    elif isinstance(feature, (np.float32, np.float64, float)):
        return 'float32'
    elif isinstance(feature, (np.int32, np.int64, six.integer_types)):
        return 'int64'
    else:
        raise ValueError('Not supported data type {}'.format(type(feature)))


def _create_table(conn, table, column_names, column_dtypes):
    drop_sql = "DROP TABLE IF EXISTS %s;" % table
    column_strs = [
        "%s %s" % (name, dtype)
        for name, dtype in zip(column_names, column_dtypes)
    ]
    create_sql = "CREATE TABLE %s (%s);" % (table, ",".join(column_strs))
    conn.execute(drop_sql)
    conn.execute(create_sql)


def xgb_shap_dataset(datasource,
                     select,
                     feature_column_names,
                     label_meta,
                     feature_metas,
                     transform_fn=None):
    conn = db.connect_with_data_source(datasource)
    stream = db.db_generator(conn, select, label_meta)
    selected_cols = db.selected_cols(conn, select)

    if transform_fn:
        feature_names = transform_fn.get_feature_column_names()
    else:
        feature_names = feature_column_names

    xs = None
    dtypes = []
    sizes = []
    offsets = []

    i = 0
    for row, label in stream():
        features = db.read_features_from_row(row,
                                             selected_cols,
                                             feature_column_names,
                                             feature_metas,
                                             is_xgboost=True)
        if transform_fn:
            features = transform_fn(features)

        flatten_features = []
        for j, feature in enumerate(features):
            if len(feature) == 3:  # convert sparse to dense
                col_indices, values, dense_shape = feature
                size = int(np.prod(dense_shape))
                row_indices = np.zeros(shape=[col_indices.size])
                sparse_matrix = scipy.sparse.csr_matrix(
                    (values, (row_indices, col_indices)), shape=[1, size])
                values = sparse_matrix.toarray()
            else:
                values = feature[0]

            if isinstance(values, np.ndarray):
                flatten_features.extend(values.flatten().tolist())
                if i == 0:
                    sizes.append(values.size)
                    dtypes.append(infer_data_type(values))
            else:
                flatten_features.append(values)
                if i == 0:
                    sizes.append(1)
                    dtypes.append(infer_data_type(values))

        # Create the column name according to the feature number
        # of each column.
        #
        # If the column "c" contains only 1 feature, the result
        # column name would be "c" too.
        #
        # If the column "c" contains 3 features,
        # the result column name would be "c_0", "c_1" and "c_2"
        if i == 0:
            offsets = np.cumsum([0] + sizes)
            column_names = []
            for j in six.moves.range(len(offsets) - 1):
                start = offsets[j]
                end = offsets[j + 1]
                if end - start == 1:
                    column_names.append(feature_names[j])
                else:
                    for k in six.moves.range(start, end):
                        column_names.append('{}_{}'.format(
                            feature_names[j], k))

            xs = pd.DataFrame(columns=column_names)

        xs.loc[i] = flatten_features

        i += 1
    # NOTE(typhoonzero): set dtype to the feature's actual type, or the dtype
    # may be "object". Use below code to reproduce:
    # import pandas as pd
    # feature_column_names=["a", "b"]
    # xs = pd.DataFrame(columns=feature_column_names)
    # for i in range(10):
    #     xs.loc[i] = [int(j) for j in range(2)]
    # print(xs.dtypes)
    columns = xs.columns
    for i, dtype in enumerate(dtypes):
        for j in six.moves.range(offsets[i], offsets[i + 1]):
            xs[columns[j]] = xs[columns[j]].astype(dtype)

    return xs


def xgb_native_explain(booster, datasource, result_table):
    if not result_table:
        raise ValueError(
            "XGBoostExplainer must use with INTO to output result to a table.")

    gain_map = booster.get_score(importance_type="gain")
    fscore_map = booster.get_fscore()
    conn = db.connect_with_data_source(datasource)

    all_feature_keys = list(gain_map.keys())
    all_feature_keys.sort()
    columns = ["feature", "fscore", "gain"]
    dtypes = [
        DataType.to_db_field_type(conn.driver, DataType.STRING),
        DataType.to_db_field_type(conn.driver, DataType.FLOAT32),
        DataType.to_db_field_type(conn.driver, DataType.FLOAT32),
    ]
    _create_table(conn, result_table, columns, dtypes)

    with db.buffered_db_writer(conn, result_table, columns) as w:
        for fkey in all_feature_keys:
            row = [fkey, fscore_map[fkey], gain_map[fkey]]
            w.write(list(row))

    conn.close()


def shap_explain(booster, datasource, select, summary_params, result_table,
                 model):
    train_fc_map = model.get_meta("features")
    label_meta = model.get_meta("label").get_field_desc()[0].to_dict(
        dtype_to_string=True)

    field_descs = get_ordered_field_descs(train_fc_map)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])

    # NOTE: in the current implementation, we are generating a transform_fn
    # from the COLUMN clause. The transform_fn is executed during the process
    # of dumping the original data into DMatrix SVM file.
    compiled_fc = compile_ir_feature_columns(train_fc_map, model.get_type())
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *compiled_fc["feature_columns"])

    dataset = xgb_shap_dataset(datasource, select, feature_column_names,
                               label_meta, feature_metas, transform_fn)

    tree_explainer = shap.TreeExplainer(booster)
    shap_values = tree_explainer.shap_values(dataset)
    if result_table:
        conn = db.connect_with_data_source(datasource)
        # TODO(typhoonzero): the shap_values is may be a
        # list of shape [3, num_samples, num_features],
        # use the first dimension here, should find out
        # when to use the other two. When shap_values is
        # not a list it can be directly used.
        if isinstance(shap_values, list):
            to_write = shap_values[0]
        else:
            to_write = shap_values

        columns = list(dataset.columns)
        dtypes = [DataType.to_db_field_type(conn.driver, DataType.FLOAT32)
                  ] * len(columns)
        _create_table(conn, result_table, columns, dtypes)
        with db.buffered_db_writer(conn, result_table, columns) as w:
            for row in to_write:
                w.write(list(row))

        conn.close()

    if summary_params.get("plot_type") == "decision":
        shap_interaction_values = tree_explainer.shap_interaction_values(
            dataset)
        expected_value = tree_explainer.expected_value
        if isinstance(shap_interaction_values, list):
            shap_interaction_values = shap_interaction_values[0]
        if isinstance(expected_value, list):
            expected_value = expected_value[0]

        plot_func = lambda: shap.decision_plot(  # noqa: E731
            expected_value,
            shap_interaction_values,
            dataset,
            show=False,
            feature_display_range=slice(None, -40, -1),
            alpha=1)
    else:
        plot_func = lambda: shap.summary_plot(  # noqa: E731
            shap_values, dataset, show=False, **summary_params)

    filename = 'summary.png'
    with temp_file.TemporaryDirectory(as_cwd=True):
        explainer.plot_and_save(plot_func, filename=filename)
        with open(filename, 'rb') as f:
            img = f.read()

    img = base64.b64encode(img)
    if six.PY3:
        img = img.decode('utf-8')
    img = "<div align='center'><img src='data:image/png;base64,%s' /></div>" \
          % img
    print(img)


def explain(datasource, select, explainer, model_params, result_table, model):
    if model_params is None:
        model_params = {}

    summary_params = dict()
    for k in model_params:
        if k.startswith("summary."):
            summary_key = k.replace("summary.", "")
            summary_params[summary_key] = model_params[k]

    if isinstance(model, six.string_types):
        model = Model.load_from_db(datasource, model)
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)

    bst = xgb.Booster()
    bst.load_model("my_model")

    if explainer == "XGBoostExplainer":
        xgb_native_explain(bst, datasource, result_table)
    else:
        # when explainer is "" or "TreeExplainer" use SHAP by default.
        shap_explain(bst, datasource, select, summary_params, result_table,
                     model)
