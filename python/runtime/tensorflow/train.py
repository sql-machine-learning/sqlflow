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

import glob
import os
import types

from runtime.model import collect_metadata
from runtime.tensorflow.get_tf_model_type import is_tf_estimator
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.set_log_level import set_log_level
from runtime.tensorflow.train_estimator import estimator_train_and_save
from runtime.tensorflow.train_keras import keras_train_and_save

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
          is_pai=False,
          pai_table="",
          pai_val_table="",
          feature_columns_code="",
          model_params_code_map={},
          model_repo_image="",
          original_sql="",
          feature_column_names_map=None):
    # NOTE(typhoonzero): feature_column_names_map is used only for PAI
    # submitter API.

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
    set_log_level(verbose, is_estimator)
    model_params.update(feature_columns)

    train_dataset_fn = get_dataset_fn(select,
                                      datasource,
                                      feature_column_names,
                                      feature_metas,
                                      label_meta,
                                      is_pai,
                                      pai_table,
                                      batch_size,
                                      epochs=epoch,
                                      shuffle_size=1000)
    val_dataset_fn = None
    if validation_select:
        val_dataset_fn = get_dataset_fn(validation_select, datasource,
                                        feature_column_names, feature_metas,
                                        label_meta, is_pai, pai_val_table,
                                        batch_size)

    if not is_estimator:  # keras
        if isinstance(estimator, types.FunctionType):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_train_and_save(estimator, model_params, save, is_pai,
                             train_dataset_fn, val_dataset_fn, label_meta,
                             epoch, verbose, validation_metrics,
                             validation_steps, load_pretrained_model,
                             model_meta)
    else:
        estimator_train_and_save(estimator, model_params, save,
                                 train_dataset_fn, val_dataset_fn, max_steps,
                                 validation_start_delay_secs,
                                 validation_throttle_secs,
                                 save_checkpoints_steps, validation_metrics,
                                 load_pretrained_model, model_meta)

    # remove cache files
    any(map(os.remove, glob.glob('cache_train.*')))
    any(map(os.remove, glob.glob('cache_validation.*')))
    print("Done training")
