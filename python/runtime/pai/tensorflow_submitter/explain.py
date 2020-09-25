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

import os
import sys

import matplotlib
import pandas as pd
import tensorflow as tf
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType, oss
from runtime.tensorflow import is_tf_estimator
from runtime.tensorflow.explain import explain_boosted_trees, explain_dnns
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import input_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import pop_optimizer_and_loss

if os.environ.get('DISPLAY', '') == '':
    print('no display found. Using non-interactive Agg backend')
    matplotlib.use('Agg')


def explain_step(datasource, select, data_table, result_table, label_column,
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

    fc_map_ir = feature_columns_code
    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.TENSORFLOW)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict()) for fd in field_descs])

    # NOTE(typhoonzero): No need to eval model_params["optimizer"] and
    # model_params["loss"] because predicting do not need these parameters.

    is_estimator = is_tf_estimator(import_model(estimator))

    # Keras single node is using h5 format to save the model, no need to deal
    # with export model format. Keras distributed mode will use estimator, so
    # this is also needed.
    model_name = oss_model_path.split("/")[-1]
    if is_estimator:
        oss.load_file(oss_model_path, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in
        # codegen/tensorflow/codegen.go
        oss.load_dir("%s/%s" % (oss_model_path, model_name))
    else:
        oss.load_dir(os.path.join(oss_model_path, "model_save"))

    # (TODO: lhw) use oss to store result image
    _explain(datasource=datasource,
             estimator_string=estimator,
             select=select,
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_metas=feature_metas,
             label_meta=label_meta,
             model_params=model_params,
             save="model_save",
             result_table=result_table,
             pai_table=data_table,
             oss_dest=None,
             oss_ak=None,
             oss_sk=None,
             oss_endpoint=None,
             oss_bucket_name=None)


def _explain(datasource,
             estimator_string,
             select,
             feature_columns,
             feature_column_names,
             feature_metas={},
             label_meta={},
             model_params={},
             save="",
             pai_table="",
             plot_type='bar',
             result_table="",
             oss_dest=None,
             oss_ak=None,
             oss_sk=None,
             oss_endpoint=None,
             oss_bucket_name=None):
    estimator_cls = import_model(estimator_string)
    FLAGS = tf.app.flags.FLAGS
    model_params["model_dir"] = FLAGS.checkpointDir
    model_params.update(feature_columns)
    pop_optimizer_and_loss(model_params)

    def _input_fn():
        dataset = input_fn("",
                           datasource,
                           feature_column_names,
                           feature_metas,
                           label_meta,
                           is_pai=True,
                           pai_table=pai_table)
        return dataset.batch(1).cache()

    estimator = init_model_with_feature_column(estimator_cls, model_params)
    conn = PaiIOConnection.from_table(result_table) if result_table else None
    if estimator_cls in (tf.estimator.BoostedTreesClassifier,
                         tf.estimator.BoostedTreesRegressor):
        explain_boosted_trees(datasource, estimator, _input_fn, plot_type,
                              result_table, feature_column_names, conn,
                              oss_dest, oss_ak, oss_sk, oss_endpoint,
                              oss_bucket_name)
    else:
        shap_dataset = pd.DataFrame(columns=feature_column_names)
        for i, (features, label) in enumerate(_input_fn()):
            shap_dataset.loc[i] = [
                item.numpy()[0][0] for item in features.values()
            ]
        explain_dnns(datasource, estimator, shap_dataset, plot_type,
                     result_table, feature_column_names, conn, oss_dest,
                     oss_ak, oss_sk, oss_endpoint, oss_bucket_name)
