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

import numpy as np
import sklearn.metrics
import xgboost as xgb
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.field_desc import DataType
from runtime.model.metadata import load_metadata
from runtime.xgboost.dataset import DMATRIX_FILE_SEP, xgb_dataset

SKLEARN_METRICS = [
    'accuracy_score',
    'average_precision_score',
    'balanced_accuracy_score',
    'brier_score_loss',
    'cohen_kappa_score',
    'explained_variance_score',
    'f1_score',
    'fbeta_score',
    'hamming_loss',
    'hinge_loss',
    'log_loss',
    'mean_absolute_error',
    'mean_squared_error',
    'mean_squared_log_error',
    'median_absolute_error',
    'precision_score',
    'r2_score',
    'recall_score',
    'roc_auc_score',
    'zero_one_loss',
]

DEFAULT_PREDICT_BATCH_SIZE = 10000


def evaluate(datasource,
             select,
             feature_metas,
             feature_column_names,
             label_meta,
             result_table,
             validation_metrics=["accuracy_score"],
             is_pai=False,
             pai_table="",
             model_params=None,
             transform_fn=None,
             feature_column_code=""):
    if not is_pai:
        conn = db.connect_with_data_source(datasource)
    else:
        conn = PaiIOConnection.from_table(pai_table)
    dpred = xgb_dataset(datasource,
                        'predict.txt',
                        select,
                        feature_metas,
                        feature_column_names,
                        label_meta,
                        is_pai,
                        pai_table,
                        True,
                        True,
                        batch_size=DEFAULT_PREDICT_BATCH_SIZE,
                        transform_fn=transform_fn,
                        feature_column_code=feature_column_code
                        )  # NOTE: default to use external memory
    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load model
    if not model_params:
        model_params = load_metadata("model_meta.json")["attributes"]
    print("Start evaluating XGBoost model...")
    feature_file_id = 0
    for pred_dmatrix in dpred:
        evaluate_and_store_result(bst, pred_dmatrix, feature_file_id,
                                  validation_metrics, model_params,
                                  feature_column_names, label_meta, is_pai,
                                  conn, result_table)
        feature_file_id += 1
    print("Done evaluating. Result table : %s" % result_table)


def evaluate_and_store_result(bst, dpred, feature_file_id, validation_metrics,
                              model_params, feature_column_names, label_meta,
                              is_pai, conn, result_table):
    preds = bst.predict(dpred)
    if model_params:
        obj = model_params["objective"]
        # binary:hinge output class labels
        if obj.startswith("binary:logistic"):
            preds = (preds > 0.5).astype(int)
        # multi:softmax output class labels
        elif obj.startswith("multi:softprob"):
            preds = np.argmax(np.array(preds), axis=1)
        # TODO(typhoonzero): deal with binary:logitraw when needed.
    else:
        # prediction output with multi-class job has two dimensions, this
        # is a temporary way, can remove this else branch when we can load
        # the model meta not only on PAI submitter.
        if len(preds.shape) == 2:
            preds = np.argmax(np.array(preds), axis=1)

    if is_pai:
        feature_file_read = open("predict.txt", "r")
    else:
        feature_file_read = open("predict.txt_%d" % feature_file_id, "r")

    y_test_list = []
    for line in feature_file_read:
        row = [i for i in line.strip().split(DMATRIX_FILE_SEP)]
        # DMatrix store label in the first column
        if label_meta["dtype"] == "float32" or label_meta[
                "dtype"] == DataType.FLOAT32:
            label = float(row[0])
        elif label_meta["dtype"] == "int64" or label_meta[
                "dtype"] == "int32" or label_meta["dtype"] == DataType.INT64:
            label = int(row[0])
        else:
            raise ValueError("unsupported label dtype: %s" %
                             label_meta["dtype"])
        y_test_list.append(label)
    y_test = np.array(y_test_list)

    evaluate_results = dict()
    for metric_name in validation_metrics:
        if metric_name not in SKLEARN_METRICS:
            raise ValueError("unsupported metric: %s" % metric_name)
        metric_func = getattr(sklearn.metrics, metric_name)
        metric_value = metric_func(y_test, preds)
        evaluate_results[metric_name] = metric_value

    # write evaluation result to result table
    result_columns = ["loss"] + validation_metrics
    with db.buffered_db_writer(conn, result_table, result_columns, 100) as w:
        row = ["0.0"]
        for mn in validation_metrics:
            row.append(str(evaluate_results[mn]))
        w.write(row)
