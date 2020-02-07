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

import xgboost as xgb
from sqlflow_submitter.db import connect_with_data_source, db_generator


def xgb_dataset(conn, fn, dataset_sql, feature_column_name, label_spec,
                feature_spec):
    if label_spec:
        label_spec["feature_name"] = label_spec["name"]
    gen = db_generator(conn.driver, conn, dataset_sql, feature_column_name,
                       label_spec, feature_spec)
    with open(fn, 'w') as f:
        for item in gen():
            if label_spec is None:
                row_data = ["%d:%f" % (i, v[0]) for i, v in enumerate(item[0])]
            else:
                features, label = item
                row_data = [str(label)] + [
                    "%d:%f" % (i, v[0]) for i, v in enumerate(features)
                ]
            f.write("\t".join(row_data) + "\n")
    # TODO(yancey1989): generate group and weight text file if necessary
    return xgb.DMatrix(fn)


def train(datasource, select, model_params, train_params, feature_field_meta,
          label_field_meta, validation_select):
    conn = connect_with_data_source(datasource)

    # NOTE(tony): sorting is necessary to achieve consistent feature orders between training job and prediction/analysis job
    feature_column_name = [k["name"] for k in feature_field_meta]
    feature_spec = {k['name']: k for k in feature_field_meta}

    dtrain = xgb_dataset(conn, 'train.txt', select, feature_column_name,
                         label_field_meta, feature_spec)
    watchlist = [(dtrain, "train")]
    if len(validation_select.strip()) > 0:
        dvalidate = xgb_dataset(conn, 'validate.txt', validation_select,
                                feature_column_name, label_field_meta,
                                feature_spec)
        watchlist.append((dvalidate, "validate"))

    re = dict()
    print("Start training XGBoost model...")
    bst = xgb.train(model_params,
                    dtrain,
                    evals=watchlist,
                    evals_result=re,
                    **train_params)
    bst.save_model("my_model")
    print("Evaluation result: %s" % re)
