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

from runtime.feature.compile import compile_ir_feature_columns
from runtime.model import EstimatorType, oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.xgboost.explain import explain as explain_xgb
from runtime.xgboost.feature_column import ComposedColumnTransformer

FLAGS = define_tf_flags()


def explain_step(datasource, select, data_table, explainer, result_table,
                 label_column, oss_model_path):
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
     fc_map_ir) = oss.load_metas(oss_model_path, "xgboost_model_desc")

    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.XGBOOST)

    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    summary_params = dict()
    for k in model_params:
        if k.startswith("summary."):
            summary_key = k.replace("summary.", "")
            summary_params[summary_key] = model_params[k]
    explain_xgb(
        datasource=datasource,
        select=select,
        feature_field_meta=feature_field_meta,
        feature_column_names=feature_column_names,
        label_meta=label_field_meta,
        summary_params=summary_params,
        explainer=explainer,
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
        feature_column_code=fc_map_ir)
