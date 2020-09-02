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

from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType, oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.xgboost.feature_column import ComposedColumnTransformer
from runtime.xgboost.predict import pred

FLAGS = define_tf_flags()


def predict_step(datasource, select, data_table, result_table, label_column,
                 oss_model_path):
    """PAI XGBoost prediction wrapper
    This function do some preparation for the local prediction, say,
    download the model from OSS, extract metadata and so on.

    Args:
        datasource: the datasource from which to get data
        select: data selection SQL statement
        data_table: tmp table which holds the data from select
        result_table: table to save prediction result
        label_column: prediction label column
        oss_model_path: the model path on OSS
    """
    # NOTE(typhoonzero): the xgboost model file "my_model" is hard coded
    # in xgboost/train.py
    oss.load_file(oss_model_path, "my_model")
    (estimator, model_params, train_params, feature_metas,
     feature_column_names, label_meta,
     fc_map_ir) = oss.load_metas(oss_model_path, "xgboost_model_desc")

    pred_label_meta = copy.copy(label_meta)
    pred_label_meta["feature_name"] = label_column

    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.XGBOOST)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict()) for fd in field_descs])

    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    pred(datasource=datasource,
         select=select,
         feature_metas=feature_metas,
         feature_column_names=feature_column_names,
         train_label_meta=label_meta,
         pred_label_meta=label_meta,
         result_table=result_table,
         is_pai=True,
         pai_table=data_table,
         model_params=model_params,
         train_params=train_params,
         transform_fn=transform_fn,
         feature_column_code=fc_map_ir)
