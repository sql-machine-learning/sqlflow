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
import copy
import io
import json
import os
import pickle
import tarfile
import tempfile

import grpc
from sqlflow_submitter import db

from .. import features
from ..proto import ir_pb2, modelzooserver_pb2, modelzooserver_pb2_grpc


def eval_attr(attr):
    try:
        return eval(attr)
    except:
        return attr


def get_xgb_kwargs(attributes, select):
    kwargs = {"model_params": {}, "train_params": {}}
    for k, v in attributes.items():
        if k.startswith("train."):
            prefix, name = k.split('.')[:2]
            if name in ("disk_cache", "epoch", "batch_size"):
                kwargs[name] = eval_attr(v.title())
            elif name in ("num_workers", ):
                # TODO(shendiaomo): pass num_workers to flags
                pass
            else:
                kwargs["train_params"][name] = eval_attr(v)
        elif k.startswith("validation."):
            kwargs[k.replace('.', '_')] = eval_attr(v)
        else:
            kwargs["model_params"][k.replace('.', '_')] = eval_attr(v)
    if "validation_select" not in kwargs:
        kwargs["validation_select"] = select
    return kwargs


def get_tf_kwargs(attributes, select):
    kwargs = {"model_params": {}}
    optimizer_args = {}
    for k, v in attributes.items():
        prefix, name = k.split('.')[:2]
        if prefix == "train":
            kwargs[name] = eval_attr(v)
        elif prefix == "validation":
            kwargs[k.replace('.', '_')] = eval_attr(v)
            if name == "metrics":
                kwargs[k.replace('.', '_')] = eval_attr(v).split(',')
        elif prefix == "model":
            kwargs["model_params"][name] = eval_attr(v)
            if name.endswith("optimizer"):
                optimizer_args.setdefault(name, {})
        elif prefix.endswith("optimizer"):
            optimizer_args.setdefault(prefix, {})[name] = eval_attr(v)
    if "validation_select" not in kwargs:
        kwargs["validation_select"] = select
    return kwargs, optimizer_args


def construct_tf_objects(model_params, opt_args):
    from tensorflow.keras.losses import (
        BinaryCrossentropy, CategoricalCrossentropy, CategoricalHinge,
        CosineSimilarity, Hinge, Huber, KLDivergence, LogCosh,
        MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError,
        MeanSquaredLogarithmicError, Poisson, SparseCategoricalCrossentropy,
        SquaredHinge)
    from tensorflow.keras.optimizers import (Adadelta, Adagrad, Adam, Adamax,
                                             Ftrl, Nadam, RMSprop, SGD)
    for name, args in opt_args.items():
        model_params.setdefault(name, 'Adagrad')
        model_params[name] = eval(model_params[name])(**args)
    if "loss" in model_params:
        model_params["loss"] = eval(model_params["loss"])()


def save_model(conn, table, *meta):
    '''
    Save the directory and specific metadata to a database table
    '''
    cursor = conn.cursor()
    db_name, tbl_name = map(lambda s: s.strip(), table.split("."))
    cursor.execute(f'CREATE DATABASE IF NOT EXISTS {db_name}')
    cursor.execute(f'DROP TABLE IF EXISTS {table}')
    cursor.execute(
        f'''CREATE TABLE {table} (id INT NOT NULL AUTO_INCREMENT KEY,
                                             block TEXT NOT NULL)''')
    pickle.dump(meta, open("meta.pkl", "wb"))
    f = io.BytesIO()
    archive = tarfile.open(None, "w|gz", f)
    archive.add(".")
    archive.close()
    f.seek(0)
    for block in iter(lambda: f.read(30000), b''):
        cursor.execute(f'INSERT INTO {table} (block) VALUES (%s)',
                       (base64.encodebytes(block), ))
    conn.commit()


def load_model(conn, model):
    '''
    Load and restore a directory and metadata that are saved in `model`
    '''
    if '/' in model:
        return load_model_from_zoo(model)

    cursor = conn.cursor()
    db_name, tbl_name = map(lambda s: s.strip(), model.split("."))
    cursor.execute(f'SELECT block FROM {model} ORDER BY id')
    with tempfile.TemporaryFile() as f:
        row = cursor.fetchone()
        while row:
            f.write(base64.b64decode(row[0]))
            row = cursor.fetchone()
        f.seek(0)
        archive = tarfile.open(None, "r|gz", f)
        archive.extractall()
        archive.close()
    return pickle.load(open("meta.pkl", "rb"))


def load_model_from_zoo(uri):
    '''
    Load and restore a directory and metadata that are saved in `uri`
    '''
    addr, name = map(lambda s: s.strip(), uri.rsplit("/", 1))
    tag = ''
    if ':' in name:
        name, tag = name.split(':')

    channel = grpc.insecure_channel(addr)
    client = modelzooserver_pb2_grpc.ModelZooServerStub(channel)
    request = modelzooserver_pb2.ReleaseModelRequest(name=name, tag=tag)
    stream = client.DownloadModel(request)
    with tempfile.TemporaryFile() as f:
        for row in stream:
            print(row)
            f.write(row.content_tar)
        f.seek(0)
        archive = tarfile.open(None, "r|gz", f)
        archive.extractall()
        archive.close()
    return pickle.load(open("meta.pkl", "rb"))


def train(statement, datasource, feature_metas, label_meta, **kwargs):
    '''Dispatch a train `statement` to appropriate engine, return metadata to be saved'''
    if statement.estimator.lower().startswith("xgboost"):
        from sqlflow_submitter.xgboost.train import train
        fc = features.get_xgb_features(feature_metas, statement.columns)
        kwargs.update(get_xgb_kwargs(statement.attributes, statement.select))
        train(datasource=datasource,
              select=statement.select,
              feature_metas=feature_metas,
              label_meta=label_meta,
              transform_fn=fc,
              feature_column_names=list(feature_metas.keys()),
              **kwargs)
        return (fc, )
    else:
        from sqlflow_submitter.tensorflow.train import train
        fc = features.get_tf_features(feature_metas, statement.columns)
        fc_names_map = {k: [f.key for f in v] for k, v in fc.items()}
        args, opt_args = get_tf_kwargs(statement.attributes, statement.select)
        kwargs.update(copy.deepcopy(args))
        construct_tf_objects(kwargs["model_params"], opt_args)
        train(datasource=datasource,
              estimator_string=statement.estimator,
              select=statement.select,
              feature_metas=feature_metas,
              label_meta=label_meta,
              feature_column_names=list(feature_metas.keys()),
              feature_columns=fc,
              save='model_save',
              **kwargs)
        return args['model_params'], opt_args, fc, fc_names_map


def predict(statement, datasource, feature_metas, label_meta, *meta, **kwargs):
    tbl_name, col_name = statement.target.rsplit('.', 1)
    if statement.estimator.lower().startswith("xgboost"):
        from sqlflow_submitter.xgboost.predict import pred
        fc, = meta
        label_meta[
            "feature_name"] = col_name  # TODO(shendiaomo): inconsistent with tf
        pred(datasource=datasource,
             select=statement.select,
             feature_metas=feature_metas,
             feature_column_names=list(feature_metas.keys()),
             label_meta=label_meta,
             transform_fn=fc,
             result_table=tbl_name,
             **kwargs)
    else:
        from sqlflow_submitter.tensorflow.predict import pred
        kwargs['model_params'], opt_args, feature_columns, fc_names_map = meta
        construct_tf_objects(kwargs['model_params'], opt_args)
        pred(datasource=datasource,
             estimator_string=statement.estimator,
             select=statement.select,
             result_table=tbl_name,
             feature_metas=feature_metas,
             feature_column_names_map=fc_names_map,
             result_col_name=col_name,
             save='model_save',
             feature_column_names=list(feature_metas.keys()),
             feature_columns=feature_columns,
             **kwargs)


def create_explain_table(conn, statement, feature_metas, label_meta):
    if statement.target:
        if statement.estimator.startswith('BoostedTrees'):
            columns = 'feature VARCHAR(255), dfc FLOAT, gain FLOAT'
        else:
            columns = ', '.join(f"{k} FLOAT" for k in feature_metas)
        cursor = conn.cursor()
        cursor.execute(f'DROP TABLE IF EXISTS {statement.target}')
        cursor.execute(f'CREATE TABLE {statement.target} ({columns})')
        conn.commit()


def create_evaluate_table(conn, statement):
    if statement.target:
        cursor = conn.cursor()
        attr = eval_attr(statement.attributes.get('validation.metrics'))
        metrics = attr.split(',') + ['loss'] if attr else ['loss']
        columns = ", ".join(f"{k} VARCHAR(255)" for k in metrics)
        cursor.execute(f'DROP TABLE IF EXISTS {statement.target}')
        cursor.execute(f'CREATE TABLE {statement.target} ({columns})')
        conn.commit()


def explain(statement, datasource, feature_metas, label_meta, *meta, **kwargs):
    if statement.estimator.lower().startswith("xgboost"):
        from sqlflow_submitter.xgboost.explain import explain
        fc, = meta
        explain(
            datasource=datasource,
            select=statement.select,
            feature_field_meta=feature_metas,
            feature_column_names=list(feature_metas.keys()),
            label_spec=label_meta,
            transform_fn=fc,
            summary_params={},  # TODO(shendiaomo): pass summary attributes
            result_table=statement.target,
            **kwargs)
    else:
        from sqlflow_submitter.tensorflow.explain import explain
        kwargs['model_params'], opt_args, feature_columns, _ = meta
        construct_tf_objects(kwargs['model_params'], opt_args)
        explain(datasource=datasource,
                estimator_string=statement.estimator,
                select=statement.select,
                feature_columns=feature_columns,
                feature_column_names=list(feature_metas.keys()),
                feature_metas=feature_metas,
                save='model_save',
                label_meta=label_meta,
                result_table=statement.target,
                **kwargs)


def evaluate(statement, datasource, feature_metas, label_meta, *meta,
             **kwargs):
    metrics = eval_attr(statement.attributes['validation.metrics']).split(',')
    if statement.estimator.lower().startswith("xgboost"):
        from sqlflow_submitter.xgboost.evaluate import evaluate
        fc, = meta
        evaluate(datasource=datasource,
                 select=statement.select,
                 feature_metas=feature_metas,
                 feature_column_names=list(feature_metas.keys()),
                 label_meta=label_meta,
                 validation_metrics=metrics,
                 transform_fn=fc,
                 result_table=statement.target,
                 **kwargs)
    else:
        from sqlflow_submitter.tensorflow.evaluate import evaluate
        kwargs['model_params'], opt_args, feature_columns, _ = meta
        construct_tf_objects(kwargs['model_params'], opt_args)
        evaluate(datasource=datasource,
                 estimator_string=statement.estimator,
                 select=statement.select,
                 feature_columns=feature_columns,
                 feature_column_names=list(feature_metas.keys()),
                 feature_metas=feature_metas,
                 save='model_save',
                 label_meta=label_meta,
                 validation_metrics=metrics,
                 result_table=statement.target,
                 **kwargs)


def show(statement, orig_sql, feature_metas, label_meta):
    print(json.dumps(["Model", "Train Statement"]))
    print(json.dumps([statement.model_save, orig_sql]))


def query(conn, select):
    cursor = conn.cursor()
    cursor.execute(select)
    if cursor.description:
        print(json.dumps([c[0] for c in cursor.description]))
        for row in cursor.fetchall():
            print(json.dumps(row))


def execute(program):
    conn = db.connect_with_data_source(program.datasource)
    for stmt in program.statements:
        if stmt.type == ir_pb2.Statement.QUERY:
            query(conn, stmt.select)
        elif stmt.type == ir_pb2.Statement.SHOW:
            _, orig_sql, specs, _ = load_model(conn, stmt.model_save)
            show(stmt, orig_sql, *specs)
        else:
            cursor = conn.cursor()
            cursor.execute(f'SELECT * FROM ({stmt.select}) t LIMIT 1')
            if not cursor.fetchone():
                raise RuntimeError("Empty dataset")
        if stmt.type == ir_pb2.Statement.TRAIN:
            specs = features.get_feature_metas(conn, stmt.select, stmt.label)
            meta = train(stmt, program.datasource, *specs)
            save_model(conn, stmt.model_save, stmt.estimator,
                       stmt.original_sql, specs, meta)
        elif stmt.type == ir_pb2.Statement.PREDICT:
            tbl, col = stmt.target.rsplit('.', 1)
            stmt.estimator, _, specs, meta = load_model(conn, stmt.model_save)
            # Create prediction table
            cursor = conn.cursor()
            cursor.execute(stmt.select)
            col_names = [i[0] for i in cursor.description]
            cursor.execute(f'DROP TABLE IF EXISTS {tbl}')
            cursor.execute(f'''CREATE TABLE {tbl} AS
                                   SELECT * FROM ({stmt.select}) t LIMIT 0''')
            old_col = specs[1]["feature_name"]  # Get label name
            if old_col != col or col not in col_names:
                t = "INT" if specs[1]["dtype"].startswith("int") else "DOUBLE"
                cursor.execute(f'ALTER TABLE {tbl} ADD COLUMN {col} {t}')
            conn.commit()
            predict(stmt, program.datasource, *specs, *meta)
        elif stmt.type == ir_pb2.Statement.EXPLAIN:
            stmt.estimator, _, specs, meta = load_model(conn, stmt.model_save)
            create_explain_table(conn, stmt, *specs)
            explain(stmt, program.datasource, *specs, *meta)
            # os.system('img2sixel summary.png')
        elif stmt.type == ir_pb2.Statement.EVALUATE:
            create_evaluate_table(conn, stmt)
            stmt.estimator, _, specs, meta = load_model(conn, stmt.model_save)
            evaluate(stmt, program.datasource, *specs, *meta)
