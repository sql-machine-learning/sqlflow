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
from sqlflow_submitter.xgboost.dataset import xgb_dataset


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
        dvalidate = xgb_dataset(datasource, 'validate.txt', validation_select,
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
