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

from runtime import db
from runtime.dbapi import table_writer
from runtime.feature.derivation import infer_feature_columns
from runtime.model.db import read_metadata_from_db
from runtime.model.model import EstimatorType, Model
from runtime.step.tensorflow.evaluate import evaluate_step as tf_evaluate
from runtime.step.tensorflow.explain import explain_step as tf_explain
from runtime.step.tensorflow.predict import predict_step as tf_pred
from runtime.step.tensorflow.train import train_step as tf_train
from runtime.step.xgboost.evaluate import evaluate as xgboost_evaluate
from runtime.step.xgboost.explain import explain as xgboost_explain
from runtime.step.xgboost.predict import pred as xgboost_pred
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
                      model_params,
                      result_table,
                      user=""):
    model = Model.load_from_db(datasource, model)
    if model.get_type() == EstimatorType.XGBOOST:
        pred_func = xgboost_pred
    else:
        pred_func = tf_pred

    pred_func(datasource=datasource,
              select=select,
              result_table=result_table,
              label_name=label_name,
              model=model)


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
    else:
        evaluate_func = tf_evaluate

    evaluate_func(datasource=datasource,
                  select=select,
                  result_table=result_table,
                  model=model,
                  label_name=label_name,
                  model_params=model_params)


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

    explain_func(datasource=datasource,
                 select=select,
                 explainer=explainer,
                 model_params=model_params,
                 result_table=result_table,
                 model=model)


def submit_local_run(datasource, select, image_name, params, into):
    print("""Execute local run.
    datasource: {},
    select: {},
    image_name: {},
    params: {},
    into: {}.""".format(datasource, select, image_name, params, into))


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
