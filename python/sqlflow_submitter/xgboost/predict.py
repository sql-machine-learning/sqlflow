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
         train_params=None):
    # TODO(typhoonzero): support running on PAI without MaxCompute AK/SK connection.
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
                        None,
                        is_pai,
                        pai_table,
                        True,
                        True,
                        batch_size=DEFAULT_PREDICT_BATCH_SIZE
                        )  # NOTE: default to use external memory
    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load data
    print("Start predicting XGBoost model...")
    feature_file_id = 0
    for pred_dmatrix in dpred:
        predict_and_store_result(bst, pred_dmatrix, feature_file_id,
                                 model_params, feature_column_names,
                                 label_name, is_pai, conn, result_table,
                                 hdfs_namenode_addr, hive_location, hdfs_user,
                                 hdfs_pass)
        feature_file_id += 1
    print("Done predicting. Predict table : %s" % result_table)


def predict_and_store_result(bst, dpred, feature_file_id, model_params,
                             feature_column_names, label_name, is_pai, conn,
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
        # prediction output wiht multi-class job has two dimensions, this is a temporary
        # way, can remove this else branch when we can load the model meta not only on PAI submitter.
        if len(preds.shape) == 2:
            preds = np.argmax(np.array(preds), axis=1)
    if is_pai:
        feature_file_read = open("predict.txt", "r")
    else:
        feature_file_read = open("predict.txt_%d" % feature_file_id, "r")

    result_column_names = feature_column_names
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
            row = [i.split(":")[1] for i in line.replace("\n", "").split("\t")]
            row.append(str(preds[line_no]))
            w.write(row)
            line_no += 1
