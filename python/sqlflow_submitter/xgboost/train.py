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
from sqlflow_submitter.tensorflow.input_fn import (pai_maxcompute_db_generator,
                                                   pai_maxcompute_input_fn)


def xgb_dataset(datasource,
                fn,
                dataset_sql,
                feature_metas,
                feature_column_names,
                label_meta,
                is_pai=False,
                pai_table=""):

    if is_pai:
        pai_table_parts = pai_table.split(".")
        formated_pai_table = "odps://%s/tables/%s" % (pai_table_parts[0],
                                                      pai_table_parts[1])
        if label_meta:
            label_column_name = label_meta['feature_name']
        else:
            label_column_name = None
        gen = pai_maxcompute_db_generator(formated_pai_table,
                                          feature_column_names,
                                          label_column_name, feature_metas)
    else:
        conn = connect_with_data_source(datasource)
        gen = db_generator(conn.driver, conn, dataset_sql,
                           feature_column_names, label_meta, feature_metas)
    with open(fn, 'w') as f:
        for item in gen():
            if label_meta is None:
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
          feature_metas,
          feature_column_names,
          label_meta,
          validation_select,
          is_pai=False,
          pai_train_table="",
          pai_validate_table=""):

    dtrain = xgb_dataset(datasource, 'train.txt', select, feature_metas,
                         feature_column_names, label_meta, is_pai,
                         pai_train_table)
    watchlist = [(dtrain, "train")]

    if len(validation_select.strip()) > 0:
        dvalidate = xgb_dataset(datasource, 'validate.txt', select,
                                feature_metas, feature_column_names,
                                label_meta, is_pai, pai_validate_table)
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
