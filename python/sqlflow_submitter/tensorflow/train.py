# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
import sys, json
import tensorflow as tf
import functools
import sys
import numpy as np
import copy
try:
    import sqlflow_models
except:
    pass
from sqlflow_submitter.db import connect_with_data_source, db_generator, parseMaxComputeDSN
from .input_fn import input_fn, pai_maxcompute_input_fn

SHUFFLE_SIZE = 1000
# TODO(shendiaomo): Remove after we fully upgrade to TF2.0
TF_VERSION_2 = True
TF_VERSION_PARTS = tf.__version__.split(".")
if int(TF_VERSION_PARTS[0]) == 1:
    TF_VERSION_2 = False

# Disable Tensorflow INFO and WARNING logs
if TF_VERSION_2:
    import logging
    tf.get_logger().setLevel(logging.ERROR)
    from .hooks import PrintStatusHook
else:
    tf.logging.set_verbosity(tf.logging.ERROR)
    from .pai_distributed import define_tf_flags, make_distributed_info_without_evaluator, dump_into_tf_config

def keras_train_and_save(estimator, model_params, save,
                         feature_column_names, feature_metas, label_meta,
                         datasource, select, validate_select,
                         batch_size, epochs, verbose):
    classifier = estimator(**model_params)
    classifier_pkg = sys.modules[estimator.__module__]
    if hasattr(classifier_pkg, "eval_metrics_fn"):
        metrics_functions = classifier_pkg.eval_metrics_fn()
        metrics = []
        for key, func in metrics_functions.items():
            func.__name__ = key
            metrics.append(func)
    else:
        metrics = ["accuracy"]

    conn = connect_with_data_source(datasource)
    # FIXME(typhoonzero): find a way to cache to local file and avoid cache lockfile already exists issue.
    train_dataset = input_fn(select, conn, feature_column_names, feature_metas, label_meta)
    train_dataset = train_dataset.shuffle(SHUFFLE_SIZE).batch(batch_size).cache()
    if validate_select != "":
        validate_dataset = input_fn(validate_select, conn, feature_column_names, feature_metas, label_meta).batch(batch_size).cache()

    classifier.compile(optimizer=classifier_pkg.optimizer(),
        loss=classifier_pkg.loss,
        metrics=metrics)
    if hasattr(classifier, 'sqlflow_train_loop'):
        classifier.sqlflow_train_loop(train_dataset)
    else:
        if label_meta["feature_name"] != "" and validate_select != "":
            history = classifier.fit(train_dataset,
                epochs=epochs if epochs else classifier.default_training_epochs(),
                validation_data=validate_dataset,
                verbose=verbose)
        else:
            history = classifier.fit(train_dataset,
                epochs=epochs if epochs else classifier.default_training_epochs(),
                verbose=verbose)
        train_keys = []
        val_keys = []
        for k in history.history.keys():
            if k.startswith("val_"):
                val_keys.append(k)
            else:
                train_keys.append(k)
        print("====== Result for training set: ======")
        for k in train_keys:
            print("%s: %s" % (k, history.history[k][-1]))
        print("====== Result for validation set: ======")
        for k in val_keys:
            print("%s: %s" % (k, history.history[k][-1]))
    classifier.save_weights(save, save_format="h5")

def estimator_train_and_save(estimator, model_params, save,
                             is_pai, FLAGS, pai_table,
                             feature_column_names, feature_metas, label_meta,
                             datasource, select, validate_select,
                             batch_size, epochs, verbose,
                             log_every_n_iter, train_max_steps, eval_start_delay_secs, eval_throttle_secs):
    classifier = estimator(**model_params)

    def train_input_fn():
        # FIXME(typhoonzero): find a way to cache to local file and avoid cache lockfile already exists issue.
        if is_pai:
            train_dataset = pai_maxcompute_input_fn(select, datasource,
                feature_column_names, feature_metas, label_meta,
                len(FLAGS.worker_hosts), FLAGS.task_index)
        else:
            conn = connect_with_data_source(datasource)
            train_dataset = input_fn(select, conn, feature_column_names, feature_metas, label_meta)
        train_dataset = train_dataset.shuffle(SHUFFLE_SIZE).batch(batch_size).cache().repeat(epochs if epochs else 1)
        return train_dataset

    if validate_select == "":
        classifier.train(input_fn=lambda:train_dataset)
    else:
        # TODO(typhoonzero): able to config metrics by calling tf.estimators.add_metrics()
        train_hooks = []
        if verbose == 1 and TF_VERSION_2:
            train_hooks = [PrintStatusHook("train", every_n_iter=log_every_n_iter)]
        train_spec = tf.estimator.TrainSpec(input_fn=lambda:train_input_fn(), max_steps=train_max_steps, hooks=train_hooks)
        eval_hooks = []
        if verbose == 1 and TF_VERSION_2:
            eval_hooks = [PrintStatusHook("eval", every_n_iter=log_every_n_iter)]
        def validate_input_fn():
            if is_pai:
                validate_dataset = pai_maxcompute_input_fn(pai_table, datasource,
                    feature_column_names, feature_metas, label_meta,
                    len(FLAGS.worker_hosts), FLAGS.task_index)
            else:
                conn = connect_with_data_source(datasource)
                validate_dataset = input_fn(validate_select, conn, feature_column_names, feature_metas, label_meta)
            validate_dataset = validate_dataset.batch(batch_size).cache()
            return validate_dataset
        eval_spec = tf.estimator.EvalSpec(input_fn=lambda:validate_input_fn(), hooks=eval_hooks, start_delay_secs=eval_start_delay_secs, throttle_secs=eval_throttle_secs)
        result = tf.estimator.train_and_evaluate(classifier, train_spec, eval_spec)
        # FIXME(typhoonzero): find out why pai will have result == None
        if not is_pai:
            print(result[0])

def train(is_keras_model,
          datasource,
          estimator,
          select,
          validate_select,
          feature_columns,
          feature_column_names,
          feature_metas={},
          label_meta={},
          model_params={},
          save="",
          batch_size=1,
          epochs=1,
          verbose=0,
          train_max_steps=None,
          eval_start_delay_secs=0,
          eval_throttle_secs=0,
          save_checkpoints_steps=100,
          log_every_n_iter=10,
          is_pai=False,
          pai_table=""):
    if is_keras_model:
        if verbose == 1:
            # show keras training progress
            tf.get_logger().setLevel(logging.INFO)
        elif verbose >= 2:
            tf.get_logger().setLevel(logging.DEBUG)
    else:
        if verbose >= 2:
            if TF_VERSION_2:
                tf.get_logger().setLevel(logging.INFO)
            else:
                tf.logging.set_verbosity(tf.logging.INFO)
    model_params.update(feature_columns)

    if is_keras_model:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        keras_train_and_save(estimator, model_params, save,
                         feature_column_names, feature_metas, label_meta,
                         datasource, select, validate_select,
                         batch_size, epochs, verbose)
    else:
        is_distributed = False
        FLAGS = None
        # only support distributed training on PAI (TF version 1.x)
        if not TF_VERSION_2:
            FLAGS = define_tf_flags()
            if len(FLAGS.worker_hosts.split(",")) > 1:
                is_distributed = True
        if is_distributed:
            cluster, task_type, task_index = make_distributed_info_without_evaluator(FLAGS)
            dump_into_tf_config(cluster, task_type, task_index)
            dist_strategy = tf.contrib.distribute.ParameterServerStrategy()
            model_params["config"] = tf.estimator.RunConfig(save_checkpoints_steps=save_checkpoints_steps,
                train_distribute=dist_strategy)
        else:
            model_params["config"] = tf.estimator.RunConfig(save_checkpoints_steps=save_checkpoints_steps)
        if is_pai:
            model_params["model_dir"] = FLAGS.checkpointDir
        else:
            model_params["model_dir"] = save
        estimator_train_and_save(estimator, model_params, save,
                             is_pai, FLAGS, pai_table,
                             feature_column_names, feature_metas, label_meta,
                             datasource, select, validate_select,
                             batch_size, epochs, verbose,
                             log_every_n_iter, train_max_steps, eval_start_delay_secs, eval_throttle_secs)

    print("Done training")

