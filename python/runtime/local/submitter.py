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

import os
import pathlib
import subprocess
import sys

from runtime import db
from runtime.dbapi import table_writer
from runtime.feature.derivation import (get_ordered_field_descs,
                                        infer_feature_columns)
from runtime.model.db import read_metadata_from_db
from runtime.model.model import EstimatorType, Model
from runtime.step.create_result_table import (create_evaluate_table,
                                              create_explain_table,
                                              create_predict_table)
from runtime.step.tensorflow.evaluate import evaluate_step as tf_evaluate
from runtime.step.tensorflow.explain import explain_step as tf_explain
from runtime.step.tensorflow.explain import print_image_as_base64_html
from runtime.step.tensorflow.predict import predict_step as tf_pred
from runtime.step.tensorflow.train import train_step as tf_train
from runtime.step.xgboost.evaluate import evaluate as xgboost_evaluate
from runtime.step.xgboost.explain import explain as xgboost_explain
from runtime.step.xgboost.predict import predict as xgboost_pred
from runtime.step.xgboost.train import train as xgboost_train


def submit_local_train(datasource,
                       original_sql,
                       select,
                       validation_select,
                       estimator_string,
                       model_image,
                       feature_column_map,
                       label_column,
                       model_params,
                       train_params,
                       validation_params,
                       save,
                       load,
                       user=""):
    """This function run train task locally.

    Args:
        datasource: string
            Like: odps://access_id:access_key@service.com/api?
                         curr_project=test_ci&scheme=http
        select: string
            The SQL statement for selecting data for train
        validation_select: string
            Ths SQL statement for selecting data for validation
        estimator_string: string
            TensorFlow estimator name, Keras class name, or XGBoost
        model_image: string
            Docker image used to train this model,
            default: sqlflow/sqlflow:step
        feature_column_map: string
            A map of Python feature column IR.
        label_column: string
            Feature column instance describing the label.
        model_params: dict
            Params for training, crossponding to WITH clause
        train_params: dict
            Extra train params, will be passed to runtime.tensorflow.train
            or runtime.xgboost.train. Optional fields:
            - disk_cache: Use dmatrix disk cache if True, default: False.
            - batch_size: Split data to batches and train, default: 1.
            - epoch: Epochs to train, default: 1.
        validation_params: dict
            Params for validation.
        save: string
            Model name to be saved.
        load: string
            The pre-trained model name to load
        user: string
            Not used for local submitter, used in runtime.pai only.
    """
    if estimator_string.lower().startswith("xgboost"):
        train_func = xgboost_train
    else:
        train_func = tf_train

    with db.connect_with_data_source(datasource) as conn:
        feature_column_map, label_column = infer_feature_columns(
            conn, select, feature_column_map, label_column, n=1000)

    return train_func(original_sql=original_sql,
                      model_image=model_image,
                      estimator_string=estimator_string,
                      datasource=datasource,
                      select=select,
                      validation_select=validation_select,
                      model_params=model_params,
                      train_params=train_params,
                      validation_params=validation_params,
                      feature_column_map=feature_column_map,
                      label_column=label_column,
                      save=save,
                      load=load)


def submit_local_pred(datasource,
                      original_sql,
                      select,
                      model,
                      label_name,
                      pred_params,
                      result_table,
                      user=""):
    model = Model.load_from_db(datasource, model)
    if model.get_type() == EstimatorType.XGBOOST:
        pred_func = xgboost_pred
    else:
        pred_func = tf_pred

    conn = db.connect_with_data_source(datasource)
    if model.get_meta("label") is None:
        train_label_desc = None
    else:
        train_label_desc = model.get_meta("label").get_field_desc()[0]

    if pred_params is None:
        extra_result_cols = []
    else:
        extra_result_cols = pred_params.get("predict.extra_outputs", "")
        extra_result_cols = [
            c.strip() for c in extra_result_cols.split(",") if c.strip()
        ]

    result_column_names, train_label_idx = create_predict_table(
        conn, select, result_table, train_label_desc, label_name,
        extra_result_cols)
    conn.close()

    pred_func(datasource=datasource,
              select=select,
              result_table=result_table,
              result_column_names=result_column_names,
              train_label_idx=train_label_idx,
              model=model,
              extra_result_cols=extra_result_cols)


def submit_local_evaluate(datasource,
                          original_sql,
                          select,
                          label_name,
                          model,
                          model_params,
                          result_table,
                          user=""):
    model = Model.load_from_db(datasource, model)
    if model.get_type() == EstimatorType.XGBOOST:
        evaluate_func = xgboost_evaluate
        validation_metrics = model_params.get("validation.metrics",
                                              "accuracy_score")
    else:
        evaluate_func = tf_evaluate
        validation_metrics = model_params.get("validation.metrics", "Accuracy")

    conn = db.connect_with_data_source(datasource)
    validation_metrics = [m.strip() for m in validation_metrics.split(",")]
    result_column_names = create_evaluate_table(conn, result_table,
                                                validation_metrics)
    conn.close()

    evaluate_func(datasource=datasource,
                  select=select,
                  result_table=result_table,
                  model=model,
                  label_name=label_name,
                  model_params=model_params,
                  result_column_names=result_column_names)


def submit_local_explain(datasource,
                         original_sql,
                         select,
                         model,
                         model_params,
                         result_table,
                         explainer="TreeExplainer",
                         user=""):
    model = Model.load_from_db(datasource, model)
    if model.get_type() == EstimatorType.XGBOOST:
        explain_func = xgboost_explain
    else:
        explain_func = tf_explain

    if result_table:
        feature_columns = model.get_meta("features")
        estimator_string = model.get_meta("class_name")
        field_descs = get_ordered_field_descs(feature_columns)
        feature_column_names = [fd.name for fd in field_descs]
        with db.connect_with_data_source(datasource) as conn:
            create_explain_table(conn, model.get_type(), explainer,
                                 estimator_string, result_table,
                                 feature_column_names)

    explain_func(datasource=datasource,
                 select=select,
                 explainer=explainer,
                 model_params=model_params,
                 result_table=result_table,
                 model=model)
    if not result_table:
        print_image_as_base64_html("summary.png")


SQLFLOW_TO_RUN_CONTEXT_KEY_SELECT = "SQLFLOW_TO_RUN_SELECT"
SQLFLOW_TO_RUN_CONTEXT_KEY_INTO = "SQLFLOW_TO_RUN_INTO"
SQLFLOW_TO_RUN_CONTEXT_KEY_IMAGE = "SQLFLOW_TO_RUN_IMAGE"


def submit_local_run(datasource, select, image_name, params, into):
    if not params:
        raise ValueError("params should not be None or empty.")

    subprocess_env = os.environ.copy()
    update_env = {
        SQLFLOW_TO_RUN_CONTEXT_KEY_SELECT: select,
        SQLFLOW_TO_RUN_CONTEXT_KEY_INTO: into,
        SQLFLOW_TO_RUN_CONTEXT_KEY_IMAGE: image_name
    }
    subprocess_env.update(update_env)

    program_file_path = pathlib.Path(params[0])
    if not program_file_path.is_file:
        raise ValueError("{} is not a file".format(params[0]))

    sub_process = None
    file_ext = program_file_path.suffix
    if not file_ext:
        args = [program_file_path]
        args.extend(params[1:])
        sub_process = subprocess.run(args=args,
                                     env=subprocess_env,
                                     stdout=subprocess.PIPE,
                                     stderr=subprocess.PIPE)
    elif file_ext.lower() == ".py":
        args = ["python", "-m", program_file_path.stem]
        args.extend(params[1:])
        sub_process = subprocess.run(args=args,
                                     env=subprocess_env,
                                     stdout=subprocess.PIPE,
                                     stderr=subprocess.PIPE)
    else:
        print(
            "The other executable except Python program is not supported yet")

    if sub_process:
        print(sub_process.stdout.decode("utf-8"))
        if sub_process.returncode != 0:
            print(sub_process.stderr.decode("utf-8"), file=sys.stderr)
            raise RuntimeError("Executing {} failed.".format(params[0]))


def submit_local_show_train(datasource, model_name):
    meta = read_metadata_from_db(datasource, model_name)
    original_sql = meta.get("original_sql")
    if not original_sql:
        raise ValueError("cannot find the train SQL statement")

    result_set = [(model_name, original_sql)]
    header = ["Model", "Train Statement"]
    writer = table_writer.ProtobufWriter(result_set, header)
    for line in writer.dump_strings():
        print(line)
