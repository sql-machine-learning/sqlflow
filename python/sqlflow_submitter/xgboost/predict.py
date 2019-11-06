# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
from sqlflow_submitter.db import connect_with_data_source, db_generator, buffered_db_writer
from .train import xgb_dataset

def pred(datasource,
         select,
         feature_field_meta,
         label_field_meta,
         result_table,
         hdfs_namenode_addr="",
         hive_location="",
         hdfs_user="",
         hdfs_pass=""):
    conn = connect_with_data_source(datasource)

    feature_column_names = [k["name"] for k in feature_field_meta]
    label_name = label_field_meta["name"]

    feature_specs = {k['name']: k for k in feature_field_meta}

    dpred = xgb_dataset(conn, 'predict.txt', select, feature_column_names, label_name, feature_specs)

    bst = xgb.Booster({'nthread': 4})  # init model
    bst.load_model("my_model")  # load data
    preds = bst.predict(dpred)

    # TODO(Yancey1989): using the train parameters to decide regressoin model or classifier model
    if len(preds.shape) == 2:
        # classifier result
        preds = np.argmax(np.array(preds), axis=1)
    feature_file_read = open("predict.txt", "r")

    result_column_names = feature_column_names
    result_column_names.append(label_name)
    line_no = 0
    with buffered_db_writer(conn.driver,
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
            row = [i.split(":")[1] for i in line.replace("\n", "").split("\t")[1:]]
            row.append(str(preds[line_no]))
            w.write(row)
            line_no += 1
    print("Done predicting. Predict table : %s" % result_table)