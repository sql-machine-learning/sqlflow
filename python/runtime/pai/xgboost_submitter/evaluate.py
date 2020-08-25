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

import runtime.xgboost as xgboost_extended
from runtime.model import oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.xgboost.evaluate import evaluate as _evaluate

FLAGS = define_tf_flags()


def evaluate(datasource, select, data_table, result_table, oss_model_path,
             metrics):
    """PAI XGBoost evaluate wrapper
    This function do some preparation for the local evaluation, say,
    download the model from OSS, extract metadata and so on.

    Args:
        datasource: the datasource from which to get data
        select: data selection SQL statement
        data_table: tmp table which holds the data from select
        result_table: table to save prediction result
        oss_model_path: the model path on OSS
        metrics: metrics to evaluate
    """

    # NOTE(typhoonzero): the xgboost model file "my_model" is hard coded
    # in xgboost/train.py
    oss.load_file(oss_model_path, "my_model")
    (estimator, model_params, train_params, feature_metas,
     feature_column_names, label_meta,
     feature_column_code) = oss.load_metas(oss_model_path,
                                           "xgboost_model_desc")

    feature_column_transformers = eval('[{}]'.format(feature_column_code))
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *feature_column_transformers)

    _evaluate(datasource=datasource,
              select=select,
              feature_metas=feature_metas,
              feature_column_names=feature_column_names,
              label_meta=label_meta,
              result_table=result_table,
              validation_metrics=metrics,
              is_pai=True,
              pai_table=data_table,
              model_params=model_params,
              transform_fn=transform_fn,
              feature_column_code=feature_column_code)
