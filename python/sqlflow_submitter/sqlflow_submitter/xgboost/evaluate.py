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
import sklearn
import xgboost as xgb
# yapf: disable
from sklearn.metrics import (accuracy_score, average_precision_score,
                             balanced_accuracy_score, brier_score_loss,
                             cohen_kappa_score, explained_variance_score,
                             f1_score, fbeta_score, hamming_loss, hinge_loss,
                             log_loss, mean_absolute_error, mean_squared_error,
                             mean_squared_log_error, median_absolute_error,
                             precision_score, r2_score, recall_score,
                             roc_auc_score, zero_one_loss)
from sqlflow_submitter import db
from sqlflow_submitter.xgboost.dataset import xgb_dataset

# yapf: enable

DEFAULT_PREDICT_BATCH_SIZE = 10000


def evaluate(datasource,
             select,
             feature_metas,
             feature_column_names,
             label_meta,
             result_table,
             validation_metrics=["accuracy_score"],
             is_pai=False,
             hdfs_namenode_addr="",
             hive_location="",
             hdfs_user="",
             hdfs_pass="",
             pai_table="",
             model_params=None):
    if not is_pai:
        conn = db.connect_with_data_source(datasource)
    else:
        conn = None
    label_name = label_meta["feature_name"]
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
                        batch_size=DEFAULT_PREDICT_BATCH_SIZE
                        )  # NOTE: default to use external memory
    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load model
    print("Start evaluating XGBoost model...")
    feature_file_id = 0
    for pred_dmatrix in dpred:
        evaluate_and_store_result(bst, pred_dmatrix, feature_file_id,
                                  validation_metrics, model_params,
                                  feature_column_names, label_meta, is_pai,
                                  conn, result_table, hdfs_namenode_addr,
                                  hive_location, hdfs_user, hdfs_pass)
        feature_file_id += 1
    print("Done evaluating. Result table : %s" % result_table)


def evaluate_and_store_result(bst, dpred, feature_file_id, validation_metrics,
                              model_params, feature_column_names, label_meta,
                              is_pai, conn, result_table, hdfs_namenode_addr,
                              hive_location, hdfs_user, hdfs_pass):
    preds = bst.predict(dpred)
    # FIXME(typhoonzero): copied from predict.py
    if model_params:
        obj = model_params["objective"]
        if obj.startswith("binary:"):
            preds = (preds > 0.5).astype(int)
        elif obj.startswith("multi:"):
            preds = np.argmax(np.array(preds), axis=1)
        else:
            # using the original prediction result of predict API by default
            pass
    else:
        # prediction output with multi-class job has two dimensions, this is a temporary
        # way, can remove this else branch when we can load the model meta not only on PAI submitter.
        if len(preds.shape) == 2:
            preds = np.argmax(np.array(preds), axis=1)

    if is_pai:
        feature_file_read = open("predict.txt", "r")
    else:
        feature_file_read = open("predict.txt_%d" % feature_file_id, "r")

    y_test_list = []
    for line in feature_file_read:
        row = [i for i in line.strip().split("\t")]
        # DMatrix store label in the first column
        if label_meta["dtype"] == "float32":
            label = float(row[0])
        elif label_meta["dtype"] == "int64" or label_meta["dtype"] == "int32":
            label = int(row[0])
        else:
            raise ValueError("unsupported label dtype: %s" %
                             label_meta["dtype"])
        y_test_list.append(label)
    y_test = np.array(y_test_list)

    evaluate_results = dict()
    for metric_name in validation_metrics:
        metric_value = eval("%s(y_test, preds)" % metric_name)
        evaluate_results[metric_name] = metric_value

    # write evaluation result to result table
    if is_pai:
        driver = "pai_maxcompute"
    else:
        driver = conn.driver
    result_columns = ["loss"] + validation_metrics
    with db.buffered_db_writer(driver,
                               conn,
                               result_table,
                               result_columns,
                               100,
                               hdfs_namenode_addr=hdfs_namenode_addr,
                               hive_location=hive_location,
                               hdfs_user=hdfs_user,
                               hdfs_pass=hdfs_pass) as w:
        row = ["0.0"]
        for mn in validation_metrics:
            row.append(str(evaluate_results[mn]))
        w.write(row)
