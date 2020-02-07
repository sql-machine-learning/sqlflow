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
from sqlflow_submitter.tensorflow.input_fn import pai_maxcompute_input_fn


def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_field_meta,
                label_spec,
                is_pai=False,
                pai_table=""):

    if label_spec:
        label_spec["feature_name"] = label_spec["name"]
    feature_column_name = [k["name"] for k in feature_field_meta]
    feature_spec = {k['name']: k for k in feature_field_meta}

    if is_pai:
        conn = connect_with_data_source(datasource)
        gen = db_generator(conn.driver, conn, dataset_sql, feature_column_name,
                           label_spec, feature_spec)
    else:
        gen = pai_maxcompute_input_fn(pai_table, datasource,
                                      feature_column_names, feature_field_meta,
                                      label_spec)
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


def train(datasource,
          select,
          model_params,
          train_params,
          feature_field_meta,
          label_field_meta,
          validation_select,
          is_pai=False,
          pai_train_table="",
          pai_validate_table=""):

    dtrain = xgb_dataset(datasource, 'train.txt', select, feature_field_meta,
                         label_field_meta, is_pai, pai_train_table)
    watchlist = [(dtrain, "train")]

    if len(validation_select.strip()) > 0:
        dvalidate = xgb_dataset(datasource, 'validate.txt', select,
                                feature_field_meta, label_field_meta, is_pai,
                                pai_validate_table)
        watchlist.append((dvalidate, "validate"))

    re = dict()
    bst = xgb.train(model_params,
                    dtrain,
                    evals=watchlist,
                    evals_result=re,
                    **train_params)
    bst.save_model("my_model")
    print("Evaluation result: %s" % re)
