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

import os
import types

from runtime.model import collect_metadata, oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
from runtime.step.tensorflow.train_estimator import estimator_train_and_save
from runtime.step.tensorflow.train_keras import keras_train_and_save
from runtime.tensorflow.get_tf_model_type import is_tf_estimator
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.set_log_level import set_log_level

# Disable TensorFlow INFO and WARNING logs
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
          is_pai=True,
          pai_table="",
          pai_val_table="",
          feature_columns_code="",
          model_params_code_map={},
          model_repo_image="",
          original_sql="",
          feature_column_names_map=None):
    # TODO(sneaxiy): collect features and label
    model_meta = collect_metadata(original_sql=original_sql,
                                  select=select,
                                  validation_select=validation_select,
                                  model_repo_image=model_repo_image,
                                  class_name=estimator_string,
                                  attributes=model_params,
                                  features=None,
                                  label=None)
    estimator = import_model(estimator_string)
    is_estimator = is_tf_estimator(estimator)

    if verbose < 1:  # always use verbose == 2 when using PAI to get INFO logs
        verbose = 2
    set_log_level(verbose, is_estimator)
    model_params.update(feature_columns)

    FLAGS = define_tf_flags()
    set_oss_environs(FLAGS)
    num_workers = len(FLAGS.worker_hosts.split(","))
    worker_id = FLAGS.task_index
    train_dataset_fn = get_dataset_fn(select,
                                      datasource,
                                      feature_column_names,
                                      feature_metas,
                                      label_meta,
                                      is_pai,
                                      pai_table,
                                      batch_size,
                                      epochs=epoch,
                                      shuffle_size=1000,
                                      num_workers=num_workers,
                                      worker_id=worker_id)
    val_dataset_fn = None
    if validation_select:
        val_dataset_fn = get_dataset_fn(validation_select, datasource,
                                        feature_column_names, feature_metas,
                                        label_meta, is_pai, pai_val_table,
                                        batch_size)

    if not is_estimator:
        if isinstance(estimator, types.FunctionType):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_train_and_save(estimator, model_params, save, FLAGS,
                             train_dataset_fn, val_dataset_fn, label_meta,
                             epoch, verbose, validation_metrics,
                             validation_steps, load_pretrained_model,
                             model_meta, is_pai)
    else:
        estimator_train_and_save(estimator, model_params, save, FLAGS,
                                 train_dataset_fn, val_dataset_fn, max_steps,
                                 validation_start_delay_secs,
                                 validation_throttle_secs,
                                 save_checkpoints_steps, validation_metrics,
                                 load_pretrained_model, model_meta)

    # save model to OSS
    if num_workers == 1 or worker_id == 0:
        oss_model_dir = FLAGS.sqlflow_oss_modeldir
        oss.save_oss_model(oss_model_dir, estimator_string, is_estimator,
                           feature_column_names, feature_column_names_map,
                           feature_metas, label_meta, model_params_code_map,
                           feature_columns_code, num_workers)
        print("Model saved to oss: %s" % oss_model_dir)
    print("Done training")
