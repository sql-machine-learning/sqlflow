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
from runtime.xgboost.explain import explain as explain_xgb

FLAGS = define_tf_flags()


def explain(datasource, select, data_table, result_table, label_column,
            oss_model_path):
    """Do XGBoost model explanation, this function use selected data to
    explain the model stored at oss_model_path

    Args:
        datasource: The datasource to load explain data
        select: SQL statement to get the data set
        data_table: tmp table to save the explain data
        result_table: table to store the explanation result
        label_column: name of the label column
        oss_model_path: path to the model to be explained
    """
    # NOTE(typhoonzero): the xgboost model file "my_model" is hard coded
    # in xgboost/train.py
    oss.load_file(oss_model_path, "my_model")

    (estimator, model_params, train_params, feature_field_meta,
     feature_column_names, label_field_meta,
     feature_column_code) = oss.load_metas(oss_model_path,
                                           "xgboost_model_desc")

    feature_column_transformers = eval('[{}]'.format(feature_column_code))
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *feature_column_transformers)

    explain_xgb(
        datasource=datasource,
        select=select,
        feature_field_meta=feature_field_meta,
        feature_column_names=feature_column_names,
        label_meta=label_field_meta,
        summary_params={},
        result_table=result_table,
        is_pai=True,
        pai_explain_table=data_table,
        # (TODO:lhw) save/load explain result storage info into/from FLAGS
        oss_dest="",
        oss_ak="",
        oss_sk="",
        oss_endpoint="",
        oss_bucket_name="",
        transform_fn=transform_fn,
        feature_column_code=feature_column_code)
