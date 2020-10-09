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
import types

import tensorflow.keras.losses as tf_loss  # noqa: F401
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import (get_ordered_field_descs,
                                        infer_feature_columns)
from runtime.model import EstimatorType, Model, collect_metadata, oss
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs
from runtime.step.tensorflow.train_estimator import estimator_train_and_save
from runtime.step.tensorflow.train_keras import keras_train_and_save
from runtime.tensorflow.get_tf_model_type import is_tf_estimator
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.set_log_level import set_log_level

if tf_is_version2():
    import tensorflow.keras.optimizers as tf_optimizers  # noqa: F401
else:
    import tensorflow.train as tf_optimizers  # noqa: F401


def get_tf_optimizer(optimizer):
    return eval("tf_optimizers." + optimizer)


def get_tf_loss(loss):
    return eval("tf_loss." + loss)


# NOTE(typhoonzero): workflow step entry for codegen/experimental,
# called by `runtime.pai.submitter` and `runtime.local.submitter`.
def train_step(original_sql,
               model_image,
               estimator_string,
               datasource,
               select,
               validation_select,
               model_params,
               train_params,
               validation_params,
               feature_column_map,
               label_column,
               save,
               load=None,
               pai_table=None,
               pai_val_table=None):
    if model_params is None:
        model_params = {}

    if train_params is None:
        train_params = {}

    if validation_params is None:
        validation_params = {}

    conn = db.connect_with_data_source(datasource)
    fc_map_ir, fc_label_ir = infer_feature_columns(conn,
                                                   select,
                                                   feature_column_map,
                                                   label_column,
                                                   n=1000)
    fc_map = compile_ir_feature_columns(fc_map_ir, EstimatorType.TENSORFLOW)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])
    label_meta = fc_label_ir.get_field_desc()[0].to_dict(dtype_to_string=True)

    feature_column_names_map = dict()
    for target in fc_map_ir:
        fclist = fc_map_ir[target]
        feature_column_names_map[target] = [
            fc.get_field_desc()[0].name for fc in fclist
        ]

    # Construct optimizer objects to pass to model initializer.
    # The original model_params is serializable (do not have tf.xxx objects).
    model_params_constructed = copy.deepcopy(model_params)
    for optimizer_arg in ["optimizer", "dnn_optimizer", "linear_optimizer"]:
        if optimizer_arg in model_params_constructed:
            model_params_constructed[optimizer_arg] = get_tf_optimizer(
                model_params_constructed[optimizer_arg])

    if "loss" in model_params_constructed:
        model_params_constructed["loss"] = get_tf_loss(
            model_params_constructed["loss"])

    # extract params for training.
    verbose = train_params.get("verbose", 1)
    batch_size = train_params.get("batch_size", 1)
    epoch = train_params.get("epoch", 1)
    save_checkpoints_steps = train_params.get("save_checkpoints_steps", 100)
    max_steps = train_params.get("max_steps", None)
    if max_steps is not None and max_steps <= 0:
        max_steps = None

    validation_metrics = validation_params.get("metrics", "Accuracy")
    validation_metrics = [v.strip() for v in validation_metrics.split(",")]
    validation_steps = validation_params.get("steps", 1)
    validation_start_delay_secs = validation_params.get("start_delay_secs", 0)
    validation_throttle_secs = validation_params.get("throttle_secs", 0)

    estimator = import_model(estimator_string)
    is_estimator = is_tf_estimator(estimator)

    is_pai = True if pai_table else False
    # always use verbose == 1 when using PAI to get more logs
    if verbose < 1:
        verbose = 1
    set_log_level(verbose, is_estimator)

    model_params_constructed.update(fc_map)

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

    model_meta = collect_metadata(original_sql=original_sql,
                                  select=select,
                                  validation_select=validation_select,
                                  model_repo_image=model_image,
                                  class_name=estimator_string,
                                  attributes=model_params,
                                  features=fc_map_ir,
                                  label=fc_label_ir)

    # FIXME(typhoonzero): avoid save model_meta twice, keras_train_and_save,
    # estimator_train_and_save also dumps model_meta to a file under cwd.
    # should only keep the model.save_to_db part.
    save_dir = "model_save"
    if not is_estimator:
        if isinstance(estimator, types.FunctionType):
            # functional model need field_metas parameter
            model_params_constructed["field_metas"] = feature_metas
        keras_train_and_save(estimator, model_params_constructed, save_dir,
                             FLAGS, train_dataset_fn, val_dataset_fn,
                             label_meta, epoch, verbose, validation_metrics,
                             validation_steps, load, model_meta, is_pai)
    else:
        estimator_train_and_save(estimator, model_params_constructed, save_dir,
                                 FLAGS, train_dataset_fn, val_dataset_fn,
                                 max_steps, validation_start_delay_secs,
                                 validation_throttle_secs,
                                 save_checkpoints_steps, validation_metrics,
                                 load, model_meta)

    # save model to DB
    if num_workers == 1 or worker_id == 0:
        if is_pai:
            oss_model_dir = FLAGS.sqlflow_oss_modeldir
            oss.save_oss_model(oss_model_dir, estimator_string, is_estimator,
                               feature_column_names, feature_column_names_map,
                               feature_metas, label_meta, model_params,
                               fc_map_ir, num_workers)
            print("Model saved to OSS: %s" % oss_model_dir)
        else:
            model = Model(EstimatorType.TENSORFLOW, model_meta)
            model.save_to_db(datasource, save)
            print("Model saved to db: %s" % save)

    print("Done training")
    conn.close()
