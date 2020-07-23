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
# limitations under the License

import copy
import json

import runtime.xgboost as xgboost_extended
from runtime import oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
from runtime.xgboost.predict import pred

FLAGS = define_tf_flags()


def predict(datasource, select, data_table, result_table, label_column,
            oss_model_path):
    # NOTE(typhoonzero): the xgboost model file "my_model" is hard coded in xgboost/train.py
    oss.load_file(oss_model_path, "my_model")
    (estimator, model_params, train_params, feature_metas,
     feature_column_names, label_meta,
     feature_column_code) = oss.load_metas(oss_model_path,
                                           "xgboost_model_desc")

    pred_label_meta = copy.copy(label_meta)
    pred_label_meta["feature_name"] = label_column

    feature_column_transformers = eval('[{}]'.format(feature_column_code))
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *feature_column_transformers)

    pred(datasource=datasource,
         select=select,
         feature_metas=feature_metas,
         feature_column_names=feature_column_names,
         train_label_meta=label_meta,
         pred_label_meta=label_meta,
         result_table=result_table,
         is_pai=True,
         hdfs_namenode_addr="",
         hive_location="",
         hdfs_user="",
         hdfs_pass="",
         pai_table=data_table,
         model_params=model_params,
         train_params=train_params,
         transform_fn=transform_fn,
         feature_column_code=feature_column_code)
