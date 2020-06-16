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
import sqlflow_submitter
import tensorflow as tf
from sqlflow_submitter.db import (connect_with_data_source, db_generator,
                                  parseMaxComputeDSN)
from sqlflow_submitter.tensorflow.get_tf_model_type import is_tf_estimator
from tensorflow.estimator import (BoostedTreesClassifier,
                                  BoostedTreesRegressor, DNNClassifier,
                                  DNNLinearCombinedClassifier,
                                  DNNLinearCombinedRegressor, DNNRegressor,
                                  LinearClassifier, LinearRegressor)

from ..model_metadata import collect_model_metadata
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

# Disable Tensorflow INFO and WARNING logs
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "3"


def train(datasource,
          estimator_string,
          select,
          validation_select,
          feature_columns,
          feature_column_names,
          feature_metas={},
          label_meta={},
          model_params={},
          validation_metrics=["Accuracy"],
          save="",
          batch_size=1,
          epoch=1,
          validation_steps=1,
          verbose=0,
          max_steps=None,
          validation_start_delay_secs=0,
          validation_throttle_secs=0,
          save_checkpoints_steps=100,
          log_every_n_iter=10,
          load_pretrained_model=False,
          is_pai=False,
          pai_table="",
          pai_val_table="",
          feature_columns_code="",
          model_repo_image=""):
    model_meta = collect_model_metadata(select, validation_select,
                                        estimator_string, model_params,
                                        feature_columns_code, feature_metas,
                                        label_meta, None, model_repo_image)
    # import custom model package
    sqlflow_submitter.import_model_def(estimator_string, globals())
    estimator = eval(estimator_string)

    is_estimator = is_tf_estimator(estimator)

    if is_pai and verbose < 1:  # always use verbose == 1 when using PAI to get more logs
        verbose = 1
    set_log_level(verbose, is_estimator)
    # fill in feature columns parameters
    model_params.update(feature_columns)

    FLAGS = None
    num_workers = 1
    worker_id = 0
    # only support distributed training on PAI (TF version 1.x)
    if is_pai:
        FLAGS = define_tf_flags()
        set_oss_environs(FLAGS)
        num_workers = len(FLAGS.worker_hosts.split(","))
        worker_id = FLAGS.task_index

    # TODO(typhoonzero): remove this after update the keras models.
    # copy feature_name to name field for Keras functional models:
    # https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier_functional_api_example.py
    for k in feature_metas:
        feature_metas[k]["name"] = feature_metas[k]["feature_name"]

    train_dataset_fn, val_dataset_fn = get_dataset_fn(
        select,
        validation_select,
        datasource,
        feature_column_names,
        feature_metas,
        label_meta,
        is_pai,
        pai_table,
        pai_val_table,
        epoch,
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
                             epoch, verbose, validation_metrics,
                             validation_steps, load_pretrained_model,
                             model_meta)
    else:
        estimator_train_and_save(estimator, model_params, save, is_pai, FLAGS,
                                 train_dataset_fn, val_dataset_fn,
                                 log_every_n_iter, max_steps,
                                 validation_start_delay_secs,
                                 validation_throttle_secs,
                                 save_checkpoints_steps, validation_metrics,
                                 load_pretrained_model, model_meta)

    # remove cache files
    any(map(os.remove, glob.glob('cache_train.*')))
    any(map(os.remove, glob.glob('cache_validation.*')))
    print("Done training")
