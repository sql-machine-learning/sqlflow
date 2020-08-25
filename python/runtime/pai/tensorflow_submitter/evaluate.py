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

import sys

import tensorflow as tf
from runtime.dbapi.paiio import PaiIOConnection
from runtime.model import oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.tensorflow import is_tf_estimator
from runtime.tensorflow.evaluate import (estimator_evaluate, keras_evaluate,
                                         write_result_metrics)
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.set_log_level import set_log_level

try:
    tf.enable_eager_execution()
except Exception as e:
    sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
    pass

FLAGS = define_tf_flags()


def evaluate(datasource, select, data_table, result_table, oss_model_path,
             metrics):
    """PAI TensorFlow evaluate wrapper
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

    (estimator, feature_column_names, feature_column_names_map, feature_metas,
     label_meta, model_params,
     feature_columns_code) = oss.load_metas(oss_model_path,
                                            "tensorflow_model_desc")

    feature_columns = eval(feature_columns_code)
    # NOTE(typhoonzero): No need to eval model_params["optimizer"] and
    # model_params["loss"] because predicting do not need these parameters.

    is_estimator = is_tf_estimator(import_model(estimator))

    # Keras single node is using h5 format to save the model, no need to deal
    # with export model format. Keras distributed mode will use estimator, so
    # this is also needed.
    if is_estimator:
        oss.load_file(oss_model_path, "exported_path")
        # NOTE(typhoonzero): directory "model_save" is hardcoded in
        # codegen/tensorflow/codegen.go
        oss.load_dir("%s/model_save" % oss_model_path)
    else:
        oss.load_file(oss_model_path, "model_save")

    _evaluate(datasource=datasource,
              estimator_string=estimator,
              select=select,
              result_table=result_table,
              feature_columns=feature_columns,
              feature_column_names=feature_column_names,
              feature_metas=feature_metas,
              label_meta=label_meta,
              model_params=model_params,
              validation_metrics=metrics,
              save="model_save",
              batch_size=1,
              validation_steps=None,
              verbose=0,
              is_pai=True,
              pai_table=data_table)


def _evaluate(datasource,
              estimator_string,
              select,
              result_table,
              feature_columns,
              feature_column_names,
              feature_metas={},
              label_meta={},
              model_params={},
              validation_metrics=["Accuracy"],
              save="",
              batch_size=1,
              validation_steps=None,
              verbose=0,
              pai_table=""):
    estimator_cls = import_model(estimator_string)
    is_estimator = is_tf_estimator(estimator_cls)
    set_log_level(verbose, is_estimator)
    eval_dataset = get_dataset_fn(select,
                                  datasource,
                                  feature_column_names,
                                  feature_metas,
                                  label_meta,
                                  is_pai=True,
                                  pai_table=pai_table,
                                  batch_size=batch_size)

    model_params.update(feature_columns)
    if is_estimator:
        FLAGS = tf.app.flags.FLAGS
        model_params["model_dir"] = FLAGS.checkpointDir
        estimator = estimator_cls(**model_params)
        result_metrics = estimator_evaluate(estimator, eval_dataset,
                                            validation_metrics)
    else:
        keras_model = init_model_with_feature_column(estimator, model_params)
        keras_model_pkg = sys.modules[estimator_cls.__module__]
        result_metrics = keras_evaluate(keras_model, eval_dataset, save,
                                        keras_model_pkg, validation_metrics)

    if result_table:
        metric_name_list = ["loss"] + validation_metrics
        write_result_metrics(result_metrics, metric_name_list, result_table,
                             PaiIOConnection.from_table(result_table))
