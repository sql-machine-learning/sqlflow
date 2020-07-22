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

import numpy as np
import runtime.db as db
import xgboost as xgb

row_data_dir = "predict.raw.dir"


def format_pred_result(objective, preds):
    if objective.startswith("binary:"):
        preds = (preds > 0.5).astype(int)
    elif objective.startswith("multi:"):
        preds = np.argmax(np.array(preds), axis=1)
    else:
        pass
    return preds


def hdfs_args():
    return {
        "hdfs_name_node_addr": os.getenv("SQLFLOW_HDFS_NAME_NODE_ADDR"),
        "hive_location": os.getenv("SQLFLOW_HIVE_LOCATION"),
        "hdfs_user": os.getenv("SQLFLOW_HDFS_USER"),
        "hdfs_pass": os.getenv("SQLFLOW_HDFS_USER"),
    }


def write_predict_result(conn, table, column_names, feature_file_idx, gen):
    with db.buffered_db_writer(conn.driver, conn, table, column_names, 100,
                               **hdfs_args()) as w:
        for row in gen():
            w.write(row)


def predict_result_columns(selected_cols, train_label_name,
                           predict_result_col):
    """Genrate the column names of prediction result table.
    """
    cols = selected_cols[:]
    if col_index(selected_cols, train_label_name) != -1:
        del cols[selected_cols.index(train_label_name)]

    cols.append(predict_result_col)
    return cols


def col_index(cols, target):
    """The target element index in the list, this function
    would return -1 if the element is not in the list.
    """
    if target in cols:
        return cols.index(target)
    return -1


def predict(model, datasource, dataset, selected_cols, result_table,
            result_col_name):
    # reload training parameters from saved model meta
    model_params = model._meta["model_params"]
    train_label_name = model._meta["train_label_name"]
    conn = db.connect_with_data_source(datasource)

    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load data
    print("Start predicting XGBoost model...")
    result_cols = predict_result_columns(selected_cols, train_label_name,
                                         result_col_name)

    for idx, per_batch_matrix in enumerate(dataset):
        preds = bst.predict(per_batch_matrix)
        preds = format_pred_result(model_params["objective"], preds)

        def _row_gen():
            skip_feature_idx = col_index(selected_cols, train_label_name)

            with open(os.path.join(row_data_dir, "predict.txt_%d" % idx),
                      "r") as f:
                for line in f:
                    row = line.strip().split("/")
                    del row[skip_feature_idx]
                    row.append(preds)
                    yield row

        write_predict_result(conn, result_table, result_cols, _row_gen)
