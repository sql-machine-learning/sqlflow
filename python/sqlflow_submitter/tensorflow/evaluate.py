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

import copy
import functools
import glob
import json
import os
import sys
import types

import numpy as np
import tensorflow as tf
from sqlflow_submitter.db import (buffered_db_writer, connect_with_data_source,
                                  db_generator, pai_maxcompute_db_generator,
                                  parseMaxComputeDSN)

from . import metrics
from .get_tf_version import tf_is_version2
from .input_fn import get_dataset_fn
from .pai_distributed import define_tf_flags, set_oss_environs
from .set_log_level import set_log_level
from .train_estimator import estimator_train_and_save
from .train_keras import keras_train_and_save

try:
    import sqlflow_models
except:
    pass


def evaluate(datasource,
             estimator_cls,
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
             hdfs_namenode_addr="",
             hive_location="",
             hdfs_user="",
             hdfs_pass="",
             is_pai=False,
             pai_table=""):
    if isinstance(estimator_cls, types.FunctionType):
        is_estimator = False
    else:
        is_estimator = issubclass(
            estimator_cls,
            (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
             tf.estimator.BoostedTreesRegressor))

    set_log_level(verbose, is_estimator)

    eval_dataset, _ = get_dataset_fn(select,
                                     "",
                                     datasource,
                                     feature_column_names,
                                     feature_metas,
                                     label_meta,
                                     is_pai,
                                     pai_table,
                                     "",
                                     1,
                                     batch_size,
                                     1,
                                     is_estimator=is_estimator)

    model_params.update(feature_columns)
    if is_estimator:
        if is_pai:
            define_tf_flags()
            FLAGS = tf.app.flags.FLAGS
            model_params["model_dir"] = FLAGS.checkpointDir
        else:
            model_params["model_dir"] = save
        estimator = estimator_cls(**model_params)
        result_metrics = estimator_evaluate(estimator, eval_dataset,
                                            validation_metrics)
    else:
        keras_model = estimator_cls(**model_params)
        keras_model_pkg = sys.modules[estimator_cls.__module__]
        result_metrics = keras_evaluate(keras_model, eval_dataset, save,
                                        keras_model_pkg, validation_metrics)

    # write result metrics to a table
    if is_pai:
        driver = "pai_maxcompute"
        conn = None
    else:
        conn = connect_with_data_source(datasource)
        driver = conn.driver

    if result_table:
        metric_name_list = ["loss"] + validation_metrics
        write_result_metrics(result_metrics,
                             metric_name_list,
                             result_table,
                             driver,
                             conn,
                             hdfs_namenode_addr=hdfs_namenode_addr,
                             hive_location=hive_location,
                             hdfs_user=hdfs_user,
                             hdfs_pass=hdfs_pass)


def estimator_evaluate(estimator, eval_dataset, validation_metrics):
    result = estimator.evaluate(eval_dataset)
    avg_loss = result["average_loss"]
    result_metrics = dict()
    result_metrics["loss"] = avg_loss
    for m in validation_metrics:
        val = result.get(m.lower())
        if val:
            result_metrics[m] = val
        else:
            # NOTE: estimator automatically append metrics for the current evaluation job,
            # if user specified metrics not appear in estimator's result dict, fill None.
            print(
                "specified metric %s not calculated by estimator, fill empty value."
                % m)
            result_metrics[m] = None

    return result_metrics


def keras_evaluate(keras_model, eval_dataset_fn, save, keras_model_pkg,
                   validation_metrics):
    # setting training metrics
    model_metrics = []
    if hasattr(keras_model_pkg, "eval_metrics_fn"):
        metrics_functions = keras_model_pkg.eval_metrics_fn()
        for key, func in metrics_functions.items():
            func.__name__ = key
            model_metrics.append(func)
    # use WITH specified metrics if it's not default.
    if validation_metrics != ["Accuracy"]:
        keras_metrics = metrics.get_keras_metrics(validation_metrics)
    else:
        if len(model_metrics) > 0:
            keras_metrics = model_metrics
        else:
            # default
            keras_metrics = metrics.get_keras_metrics(["Accuracy"])

    # compile the model with default arguments only for evaluation (run forward only).
    keras_model.compile(loss=keras_model_pkg.loss, metrics=keras_metrics)

    eval_dataset = eval_dataset_fn()

    def get_features(sample, label):
        return sample

    def get_label(sample, label):
        return label

    eval_dataset_x = eval_dataset.map(get_features)
    eval_dataset_y = eval_dataset.map(get_label)

    one_batch = next(iter(eval_dataset_x))
    # NOTE: must run predict one batch to initialize parameters
    # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models
    keras_model.predict_on_batch(one_batch)
    keras_model.load_weights(save)
    result = keras_model.evaluate(eval_dataset)
    assert (len(result) == len(validation_metrics) + 1)
    result_metrics = dict()
    for idx, m in enumerate(["loss"] + validation_metrics):
        result_metrics[m] = result[idx]
    return result_metrics


def write_result_metrics(result_metrics, metric_name_list, result_table,
                         driver, conn, hdfs_namenode_addr, hive_location,
                         hdfs_user, hdfs_pass):
    # NOTE: assume that the result table is already created with columns:
    # loss | metric_names ...
    column_names = metric_name_list
    with buffered_db_writer(driver, conn, result_table, column_names, 100,
                            hdfs_namenode_addr, hive_location, hdfs_user,
                            hdfs_pass) as w:
        row = []
        for key in metric_name_list:
            row.append(result_metrics[key])
        w.write(row)
