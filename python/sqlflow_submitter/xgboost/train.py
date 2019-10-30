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

import xgboost as xgb
from sqlflow_submitter.db import connect_with_data_source, db_generator

def xgb_dataset(conn, fn, dataset_sql, feature_column_name, label_name, feature_spec):
    gen = db_generator(conn.driver, conn, dataset_sql, feature_column_name, label_name, feature_spec)
    with open(fn, 'w') as f:
        for item in gen():
            features, label = item
            row_data = [str(label[0])] + ["%d:%f" % (i, v) for i, v in enumerate(features)]
            f.write("\t".join(row_data) + "\n")
    # TODO(yancey1989): generate group and weight text file if necessary
    return xgb.DMatrix(fn)

def train(datasource, select, model_params, train_params, feature_field_meta, label_field_meta):
    conn = connect_with_data_source(datasource)

    # NOTE(tony): sorting is necessary to achieve consistent feature orders between training job and prediction/analysis job
    feature_column_name = sorted([k["name"] for k in feature_field_meta])
    label_name = label_field_meta["name"]
    feature_spec = {k['name']: k for k in feature_field_meta}

    dtrain = xgb_dataset(conn, 'train.txt', select, feature_column_name, label_name, feature_spec)
    # FIXME(weiguoz): bring dtest back when VALIDATE clause is ready
    # dtest = xgb_dataset('test.txt', '''{{.ValidationSelect}}''')

    bst = xgb.train(model_params, dtrain, **train_params)
    bst.save_model("my_model")