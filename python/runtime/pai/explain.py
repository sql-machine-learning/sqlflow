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

import json
import os
import sys
import types

import matplotlib
import tensorflow as tf
from runtime import oss
from runtime.tensorflow import explain, is_tf_estimator
from tensorflow.estimator import (BoostedTreesClassifier,
                                  BoostedTreesRegressor, DNNClassifier,
                                  DNNLinearCombinedClassifier,
                                  DNNLinearCombinedRegressor, DNNRegressor,
                                  LinearClassifier, LinearRegressor)

try:
    from runtime.pai import model
    from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
except:
    pass  # PAI is not always needed

if os.environ.get('DISPLAY', '') == '':
    print('no display found. Using non-interactive Agg backend')
    matplotlib.use('Agg')


def explain_tf(datasource, select, data_table, result_table, label_column,
               oss_model_path):
    try:
        tf.enable_eager_execution()
    except Exception as e:
        sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
        pass

    (estimator, feature_column_names, feature_column_names_map, feature_metas,
     label_meta, model_params,
     feature_columns_code) = oss.load_metas(oss_model_path,
                                            "tensorflow_model_desc")

    feature_columns = eval(feature_columns_code)
    # NOTE(typhoonzero): No need to eval model_params["optimizer"] and model_params["loss"]
    # because predicting do not need these parameters.

    is_estimator = is_tf_estimator(eval(estimator))

    # Keras single node is using h5 format to save the model, no need to deal with export model format.
    # Keras distributed mode will use estimator, so this is also needed.
    if is_estimator:
        oss.load_file(oss_model_path, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in codegen/tensorflow/codegen.go
        oss.load_dir("%s/model_save" % oss_model_path)
    else:
        oss.load_file(oss_model_path, "model_save")

    #(TODO: lhw) use oss to store result image
    explain.explain(datasource=datasource,
                    estimator_string=estimator,
                    select=select,
                    feature_columns=feature_columns,
                    feature_column_names=feature_column_names,
                    feature_metas=feature_metas,
                    label_meta=label_meta,
                    model_params=model_params,
                    save="model_save",
                    result_table=result_table,
                    is_pai=True,
                    pai_table=data_table,
                    oss_dest=None,
                    oss_ak=None,
                    oss_sk=None,
                    oss_endpoint=None,
                    oss_bucket_name=None)
