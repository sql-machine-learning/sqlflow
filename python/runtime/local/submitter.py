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

from runtime.local.xgboost_submitter.evaluate import \
    evaluate as xgboost_evaluate
from runtime.local.xgboost_submitter.predict import pred as xgboost_pred
from runtime.local.xgboost_submitter.train import train as xgboost_train
from runtime.model.model import EstimatorType, Model


def submit_local_train(datasource, estimator_string, select, validation_select,
                       model_params, save, load, train_params):
    """This function run train task locally.

    Args:
        datasource: string
            Like: odps://access_id:access_key@service.com/api?
                         curr_project=test_ci&scheme=http
        estimator_string: string
            TensorFlow estimator name, Keras class name, or XGBoost
        select: string
            The SQL statement for selecting data for train
        validation_select: string
            Ths SQL statement for selecting data for validation
        model_params: dict
            Params for training, crossponding to WITH clause
        load: string
            The pre-trained model name to load
        train_params: dict
            Extra train params, will be passed to runtime.tensorflow.train
            or runtime.xgboost.train. Required fields:
            - original_sql: Original SQLFlow statement.
            - model_image: Docker image used for training.
            - feature_column_map: A map of Python feature column IR.
            - label_column: Feature column instance describing the label.
            - disk_cache (optional): Use dmatrix disk cache if True.
            - batch_size (optional): Split data to batches and train.
            - epoch (optional): Epochs to train.
    """
    if estimator_string.lower().startswith("xgboost"):
        # pop required params from train_params
        original_sql = train_params.pop("original_sql")
        model_image = train_params.pop("model_image")
        feature_column_map = train_params.pop("feature_column_map")
        label_column = train_params.pop("label_column")

        return xgboost_train(original_sql,
                             model_image,
                             estimator_string,
                             datasource,
                             select,
                             validation_select,
                             model_params,
                             train_params,
                             feature_column_map,
                             label_column,
                             save,
                             load=load)
    else:
        raise NotImplementedError("not implemented model type: %s" %
                                  estimator_string)


def submit_local_pred(datasource, select, result_table, pred_label_name, load):
    model = Model.load_from_db(datasource, load)
    if model.get_type() == EstimatorType.XGBOOST:
        xgboost_pred(datasource, select, result_table, pred_label_name, model)
    else:
        raise NotImplementedError("not implemented model type: {}".format(
            model.get_type()))


def submit_local_evaluate(datasource, select, result_table, pred_label_name,
                          load, validation_metrics):
    model = Model.load_from_db(datasource, load)
    if model.get_type() == EstimatorType.XGBOOST:
        xgboost_evaluate(datasource, select, result_table, model,
                         pred_label_name, validation_metrics)
    else:
        raise NotImplementedError("not implemented model type: {}".format(
            model.get_type()))
