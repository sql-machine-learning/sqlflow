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
from sqlflow_submitter.db import (connect_with_data_source, db_generator,
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


def train(datasource,
          estimator,
          select,
          validate_select,
          feature_columns,
          feature_column_names,
          feature_metas={},
          label_meta={},
          model_params={},
          metric_names=["Accuracy"],
          save="",
          batch_size=1,
          epochs=1,
          validation_steps=1,
          verbose=0,
          train_max_steps=None,
          eval_start_delay_secs=0,
          eval_throttle_secs=0,
          save_checkpoints_steps=100,
          log_every_n_iter=10,
          is_pai=False,
          pai_table="",
          pai_val_table=""):
    if isinstance(estimator, types.FunctionType):
        is_estimator = False
    else:
        is_estimator = issubclass(
            estimator,
            (tf.estimator.Estimator, tf.estimator.BoostedTreesClassifier,
             tf.estimator.BoostedTreesRegressor))
    if is_pai and verbose < 1:  # always use verbose == 1 when using PAI to get more logs
        verbose = 1
    set_log_level(verbose, is_estimator)
    # fill in feature columns parameters
    model_params.update(feature_columns)

    FLAGS = None
    is_distributed = False
    num_workers = 1
    worker_id = 0
    # only support distributed training on PAI (TF version 1.x)
    if is_pai:
        FLAGS = define_tf_flags()
        set_oss_environs(FLAGS)
        if len(FLAGS.worker_hosts.split(",")) > 1:
            is_distributed = True
        num_workers = len(FLAGS.worker_hosts.split(","))
        worker_id = FLAGS.task_index

    # TODO(typhoonzero): remove this after update the keras models.
    # copy feature_name to name field for Keras functional models:
    # https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier_functional_api_example.py
    for k in feature_metas:
        feature_metas[k]["name"] = feature_metas[k]["feature_name"]

    train_dataset_fn, val_dataset_fn = get_dataset_fn(
        select,
        validate_select,
        datasource,
        feature_column_names,
        feature_metas,
        label_meta,
        is_pai,
        pai_table,
        pai_val_table,
        epochs,
        batch_size,
        1000,
        num_workers=num_workers,
        worker_id=worker_id,
        is_estimator=is_estimator)

    if not is_estimator:  # keras
        if isinstance(estimator, types.FunctionType):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                             train_dataset_fn, val_dataset_fn, label_meta,
                             epochs, verbose, metric_names, validation_steps)
    else:
        estimator_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                                 train_dataset_fn, val_dataset_fn,
                                 log_every_n_iter, train_max_steps,
                                 eval_start_delay_secs, eval_throttle_secs,
                                 save_checkpoints_steps, metric_names)

    # remove cache files
    any(map(os.remove, glob.glob('cache_train.*')))
    print("Done training")
