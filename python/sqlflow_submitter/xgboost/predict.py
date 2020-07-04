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
import xgboost as xgb
from sqlflow_submitter import db
from sqlflow_submitter.xgboost.dataset import xgb_dataset

DEFAULT_PREDICT_BATCH_SIZE = 10000


def pred(datasource,
         select,
         feature_metas,
         feature_column_names,
         label_meta,
         result_table,
         is_pai=False,
         hdfs_namenode_addr="",
         hive_location="",
         hdfs_user="",
         hdfs_pass="",
         pai_table="",
         model_params=None,
         train_params=None,
         transform_fn=None,
         feature_column_code=""):
    if not is_pai:
        conn = db.connect_with_data_source(datasource)
    else:
        conn = None
    label_name = label_meta["feature_name"]
    dpred = xgb_dataset(
        datasource=datasource,
        fn='predict.txt',
        dataset_sql=select,
        feature_specs=feature_metas,
        feature_column_names=feature_column_names,
        label_spec=None,
        is_pai=is_pai,
        pai_table=pai_table,
        pai_single_file=True,
        cache=True,
        batch_size=DEFAULT_PREDICT_BATCH_SIZE,
        transform_fn=transform_fn,
        feature_column_code=feature_column_code,
        raw_data_dir="predict.raw.dir")  # NOTE: default to use external memory
    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load data
    print("Start predicting XGBoost model...")

    if is_pai:
        pai_table = "odps://{}/tables/{}".format(*pai_table.split("."))
        selected_cols = db.pai_selected_cols(pai_table)
    else:
        selected_cols = db.selected_cols(conn.driver, conn, select)

    feature_file_id = 0
    for pred_dmatrix in dpred:
        predict_and_store_result(bst, pred_dmatrix, feature_file_id,
                                 model_params, selected_cols, label_name,
                                 is_pai, conn, result_table,
                                 hdfs_namenode_addr, hive_location, hdfs_user,
                                 hdfs_pass)
        feature_file_id += 1
    print("Done predicting. Predict table : %s" % result_table)


def predict_and_store_result(bst, dpred, feature_file_id, model_params,
                             selected_cols, label_name, is_pai, conn,
                             result_table, hdfs_namenode_addr, hive_location,
                             hdfs_user, hdfs_pass):
    preds = bst.predict(dpred)

    #TODO(yancey1989): should save train_params and model_params not only on PAI submitter
    #TODO(yancey1989): output the original result for various objective function.
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
        feature_file_read = open("predict.txt.raw", "r")
    else:
        feature_file_read = open(
            "predict.raw.dir/predict.txt_%d" % feature_file_id, "r")

    result_column_names = selected_cols

    # Users may use "SELECT ..., label ... TO PREDICT new_table.new_label" to
    # write both the actual label and the prediction label into the result
    # table for comparision. So if "new_label == label", we should use
    # "INSERT INTO new_table (..., label) VALUES ..." to write the result table,
    # and if new_label != label, we should use
    # "INSERT INTO new_table (..., label, new_label) VALUES..." to write the result table.
    # "new_label == label" is equivalent to "label_name in selected_cols" .
    label_index = selected_cols.index(
        label_name) if label_name in selected_cols else None
    if label_index is None:
        result_column_names.append(label_name)

    line_no = 0
    if is_pai:
        driver = "pai_maxcompute"
    else:
        driver = conn.driver
    with db.buffered_db_writer(driver,
                               conn,
                               result_table,
                               result_column_names,
                               100,
                               hdfs_namenode_addr=hdfs_namenode_addr,
                               hive_location=hive_location,
                               hdfs_user=hdfs_user,
                               hdfs_pass=hdfs_pass) as w:
        while True:
            line = feature_file_read.readline()
            if not line:
                break
            row = [
                item for i, item in enumerate(line.strip().split("/"))
                if i != label_index
            ]
            row.append(str(preds[line_no]))
            w.write(row)
            line_no += 1
