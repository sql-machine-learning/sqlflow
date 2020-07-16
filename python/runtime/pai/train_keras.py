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

import inspect
import sys
import warnings
from os import path

import six
import tensorflow as tf
from runtime import oss
from runtime.model_metadata import save_model_metadata
from runtime.pai.pai_distributed import (
    dump_into_tf_config, make_distributed_info_without_evaluator)
from runtime.seeding import get_tf_random_seed
from runtime.tensorflow import metrics
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.input_fn import input_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.train_estimator import estimator_train_compiled
from runtime.tensorflow.train_keras import keras_compile, keras_train_compiled


def keras_train_and_save(estimator, model_params, save, FLAGS,
                         train_dataset_fn, val_dataset_fn, label_meta, epochs,
                         verbose, metric_names, validation_steps,
                         load_pretrained_model, model_meta):
    print("Start training using keras model...")
    classifier, has_none_optimizer = keras_compile(estimator, model_params,
                                                   save, metric_names)
    train_dataset = train_dataset_fn()
    if val_dataset_fn != None:
        validate_dataset = val_dataset_fn()
    else:
        validate_dataset = None

    if load_pretrained_model:
        # FIXME(typhoonzero): copied from runtime.tensorflow.train_keras
        inputs, targets = next(iter(train_dataset.take(1)))
        classifier.evaluate(inputs, targets)
        classifier.load_weights(save)

    if len(FLAGS.worker_hosts.split(",")) > 1:
        keras_train_distributed(classifier, model_params, save, model_meta,
                                FLAGS, train_dataset_fn, val_dataset_fn)
    else:
        keras_train_compiled(classifier, save, train_dataset, validate_dataset,
                             label_meta, epochs, verbose, model_meta,
                             has_none_optimizer)

    print("saving keras model to: %s" % FLAGS.sqlflow_oss_modeldir)
    oss.save_file(FLAGS.sqlflow_oss_modeldir, save)
    oss.save_file(FLAGS.sqlflow_oss_modeldir, "model_meta.json")


def keras_train_distributed(classifier,
                            model_params,
                            save,
                            model_meta,
                            FLAGS,
                            train_dataset_fn,
                            val_dataset_fn,
                            is_pai=True):
    # train keras model distributed
    cluster, task_type, task_index = make_distributed_info_without_evaluator(
        FLAGS)
    dump_into_tf_config(cluster, task_type, task_index)
    dist_strategy = tf.contrib.distribute.ParameterServerStrategy()

    run_config = tf.estimator.RunConfig(tf_random_seed=get_tf_random_seed(),
                                        save_checkpoints_steps=100,
                                        train_distribute=dist_strategy,
                                        session_config=tf.ConfigProto(
                                            log_device_placement=True,
                                            device_filters=None))
    model_dir = FLAGS.checkpointDir

    keras_estimator = tf.keras.estimator.model_to_estimator(
        classifier, model_dir=model_dir, config=run_config)
    estimator_train_compiled(
        keras_estimator,
        is_pai,
        FLAGS,
        train_dataset_fn,
        val_dataset_fn,
        # TODO(typhoonzero): do pass train settings.
        100,
        None,
        60,
        120)
    # FIXME(typhoonzero): predict keras distributed model should also call model_to_estimator.
    # export saved model for prediction
    if "feature_columns" in model_params:
        all_feature_columns = model_params["feature_columns"]
    elif "linear_feature_columns" in model_params and "dnn_feature_columns" in model_params:
        import copy
        all_feature_columns = copy.copy(model_params["linear_feature_columns"])
        all_feature_columns.extend(model_params["dnn_feature_columns"])
    else:
        raise Exception("No expected feature columns in model params")
    serving_input_fn = tf.estimator.export.build_parsing_serving_input_receiver_fn(
        tf.feature_column.make_parse_example_spec(all_feature_columns))
    export_path = keras_estimator.export_saved_model(save, serving_input_fn)

    # write the path under current directory
    export_path_str = str(export_path.decode("utf-8"))
    with open("exported_path", "w") as fn:
        fn.write(export_path_str)
    # write model metadata to model_meta.json
    save_model_metadata("model_meta.json", model_meta)
    print("Done training, model exported to: %s" % export_path_str)
