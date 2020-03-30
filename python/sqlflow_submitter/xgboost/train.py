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
          disk_cache=False,
          batch_size=None,
          epoch=1,
          is_pai=False,
          pai_train_table="",
          pai_validate_table=""):

    if batch_size == -1:
        batch_size = None
    print("Start training XGBoost model...")
    dtrain = xgb_dataset(datasource,
                         'train.txt',
                         select,
                         feature_metas,
                         feature_column_names,
                         label_meta,
                         is_pai,
                         pai_train_table,
                         cache=disk_cache,
                         batch_size=batch_size,
                         epoch=epoch)
    bst = None
    for per_batch_dmatrix in dtrain:
        watchlist = [(per_batch_dmatrix, "train")]
        if len(validation_select.strip()) > 0:
            dvalidate = list(
                xgb_dataset(datasource, 'validate.txt', validation_select,
                            feature_metas, feature_column_names, label_meta,
                            is_pai, pai_validate_table))[0]
            watchlist.append((dvalidate, "validate"))

        re = dict()
        bst = xgb.train(model_params,
                        per_batch_dmatrix,
                        evals=watchlist,
                        evals_result=re,
                        xgb_model=bst,
                        **train_params)
        bst.save_model("my_model")
        print("Evaluation result: %s" % re)
