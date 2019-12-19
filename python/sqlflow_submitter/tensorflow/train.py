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
# Disable Tensorflow INFO and WARNING logs
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

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

TF_VERSION_2 = True  # TODO(shendiaomo): Remove after we fully upgrade to TF2.0
# Disable Tensorflow INFO and WARNING
try:
    if tf.version.VERSION > '1':
        import logging
        tf.get_logger().setLevel(logging.ERROR)
    else:
        raise ImportError
except:
    tf.logging.set_verbosity(tf.logging.ERROR)
    TF_VERSION_2 = False

FLAGS = None

# ----------------- For PAI distributed training -----------------
def define_tf_flags():
    global FLAGS
    if not TF_VERSION_2:
        tf.app.flags.DEFINE_integer("task_index", 0, "Worker task index")
        tf.app.flags.DEFINE_string("ps_hosts", "", "ps hosts")
        tf.app.flags.DEFINE_string("worker_hosts", "", "worker hosts")
        tf.app.flags.DEFINE_string("job_name", 'worker', "job name: worker or ps")
        tf.app.flags.DEFINE_string("checkpointDir", "", "oss info")
        tf.app.flags.DEFINE_string('model_dir', './output', 'model directory')
        FLAGS = tf.app.flags.FLAGS

# make_distributed_info_without_evaluator and dump_into_tf_config are used to dump
# distributed configurations into environment variable TF_CONFIG so that Tensorflow
# can recognize it.
def make_distributed_info_without_evaluator():
    global FLAGS
    worker_hosts = FLAGS.worker_hosts.split(",")
    ps_hosts = FLAGS.ps_hosts.split(",")
    if len(worker_hosts) > 1:
        cluster = {"chief": [worker_hosts[0]],
               "worker": worker_hosts[1:],
               "ps": ps_hosts}
    else:
        cluster = {"chief": [worker_hosts[0]],
               "ps": ps_hosts}

    if FLAGS.job_name == "worker":
        if FLAGS.task_index == 0:
            task_type = "chief"
            task_index = 0
        else:
            task_type = "worker"
            task_index = FLAGS.task_index - 1
    else:
        task_type = "ps"
        task_index = FLAGS.task_index
    return cluster, task_type, task_index

def dump_into_tf_config(cluster, task_type, task_index):
  os.environ['TF_CONFIG'] = json.dumps(
      {'cluster': cluster,
       'task': {'type': task_type, 'index': task_index}})  
# ----------------- For PAI distributed training -----------------

def get_dtype(type_str):
    if type_str == "float32":
        return tf.float32
    elif type_str == "int64":
        return tf.int64
    else:
        raise TypeError("not supported dtype: %s" % type_str)

def parse_sparse_feature(features, label, feature_column_names, feature_metas):
    features_dict = dict()
    for idx, col in enumerate(features):
        name = feature_column_names[idx]
        if feature_metas[name]["is_sparse"]:
            i, v, s = col
            features_dict[name] = tf.SparseTensor(indices=i, values=v, dense_shape=s)
        else:
            features_dict[name] = col
    return features_dict, label

if TF_VERSION_2:
    class PrintStatusHook(tf.estimator.LoggingTensorHook):
        def __init__(self, prefix="", every_n_iter=None, every_n_secs=None,
                at_end=False, formatter=None):
            super().__init__([], every_n_iter=every_n_iter, every_n_secs=every_n_secs,
                at_end=at_end, formatter=formatter)
            self.prefix = prefix

        def before_run(self, run_context):
            self._should_trigger = self._timer.should_trigger_for_step(self._iter_count)
            loss_vars = tf.compat.v1.get_collection(tf.compat.v1.GraphKeys.LOSSES)
            step_vars = tf.compat.v1.get_collection(tf.compat.v1.GraphKeys.GLOBAL_STEP)
            fetch = {"loss": loss_vars[0], "step": step_vars[0]}
            if self._should_trigger:
                return tf.estimator.SessionRunArgs(fetch)
            else:
                return None

        def _log_tensors(self, tensor_values):
            elapsed_secs, _ = self._timer.update_last_triggered_step(self._iter_count)
            stats = []
            for k in tensor_values.keys():
                stats.append("%s = %s" % (k, tensor_values[k]))
            if self.prefix == "eval":
                print("============Evaluation=============")
            print("%s: %s" % (self.prefix, ", ".join(stats)))
            if self.prefix == "eval":
                print("============Evaluation End=============")

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
    global FLAGS
    define_tf_flags()
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
    if not is_pai:
        conn = connect_with_data_source(datasource)
    model_params.update(feature_columns)

    def input_fn(datasetStr):
        feature_types = []
        for name in feature_column_names:
            # NOTE: vector columns like 23,21,3,2,0,0 should use shape None
            if feature_metas[name]["is_sparse"]:
                feature_types.append((tf.int64, tf.int32, tf.int64))
            else:
                feature_types.append(get_dtype(feature_metas[name]["dtype"]))

        gen = db_generator(conn.driver, conn, datasetStr, feature_column_names, label_meta["feature_name"], feature_metas)
        dataset = tf.data.Dataset.from_generator(gen, (tuple(feature_types), eval("tf.%s" % label_meta["dtype"])))
        ds_mapper = functools.partial(parse_sparse_feature, feature_column_names=feature_column_names, feature_metas=feature_metas)
        return dataset.map(ds_mapper)

    def pai_maxcompute_input_fn(datasetStr, num_workers=1, worker_id=0):
        table_parts = pai_table.split(".")
        if len(table_parts) == 2:
            database, table_name = table_parts
        elif len(table_parts) == 1:
            table_name = pai_table
            driver, dsn = datasource.split("://")
            database = parseMaxComputeDSN(dsn)[-1]
        else:
            raise ValueError("error database.table format: %s" % pai_table)

        tables = ["odps://%s/tables/%s" % (database, table_name)]
        record_defaults = []
        for name in feature_column_names:
            dtype = get_dtype(feature_metas[name]["dtype"])
            record_defaults.append(tf.constant(0, dtype=dtype, shape=feature_metas[name]["shape"]))
        record_defaults.append(
            tf.constant(0, get_dtype(label_meta["dtype"]), shape=label_meta["shape"]))

        selected_cols = copy.copy(feature_column_names)
        selected_cols.append(label_meta["feature_name"])
        if num_workers == 0:
            num_workers = 1
        dataset = tf.data.TableRecordDataset(tables,
                                     record_defaults=record_defaults,
                                     selected_cols=",".join(selected_cols),
                                     slice_id=worker_id,
                                     slice_count=num_workers)
        def tensor_to_dict(*args):
            num_features = len(feature_column_names)
            label = args[num_features]
            features_dict = dict()
            for idx in range(num_features):
                name = feature_column_names[idx]
                features_dict[name] = tf.reshape(args[idx], [-1])
            return features_dict, label

        return dataset.map(tensor_to_dict)

    def train_input_fn(batch_size):
        if is_pai:
            dataset = pai_maxcompute_input_fn(select, len(FLAGS.worker_hosts), FLAGS.task_index)
        else:
            dataset = input_fn(select)
        # FIXME(typhoonzero): find a way to cache to local file and avoid cache lockfile already exists issue.
        dataset = dataset.shuffle(1000).batch(batch_size).cache()
        if not is_keras_model:
            dataset = dataset.repeat(epochs if epochs else 1)
        return dataset

    def validate_input_fn(batch_size):
        if is_pai:
            dataset = pai_maxcompute_input_fn(validate_select, len(FLAGS.worker_hosts), FLAGS.task_index)
        else:
            dataset = input_fn(validate_select)
        return dataset.batch(batch_size).cache()

    if is_keras_model:
        if not issubclass(estimator, tf.keras.Model):
            # functional model need field_metas parameter
            model_params["field_metas"] = feature_metas
        classifier = estimator(**model_params)
        classifier_pkg = sys.modules[estimator.__module__]

        classifier.compile(optimizer=classifier_pkg.optimizer(),
            loss=classifier_pkg.loss,
            metrics=["accuracy"])
        if hasattr(classifier, 'sqlflow_train_loop'):
            classifier.sqlflow_train_loop(train_input_fn(batch_size))
        else:
            ds = train_input_fn(batch_size)
            if label_meta["feature_name"] != "" and validate_select != "":
                history = classifier.fit(ds,
                    epochs=epochs if epochs else classifier.default_training_epochs(),
                    validation_data=validate_input_fn(batch_size),
                    verbose=verbose)
            else:
                history = classifier.fit(ds,
                    epochs=epochs if epochs else classifier.default_training_epochs(),
                    verbose=verbose)
            for k, v in history.history.items():
                print("%s: %s" % (k, v[-1]))
        classifier.save_weights(save, save_format="h5")
    else:
        is_distributed = False
        # only support distributed training on PAI (TF version 1.x)
        if not TF_VERSION_2:
            if len(FLAGS.worker_hosts.split(",")) > 1:
                is_distributed = True
        if is_distributed:
            cluster, task_type, task_index = make_distributed_info_without_evaluator()
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
        classifier = estimator(**model_params)

        if validate_select == "":
            classifier.train(input_fn=lambda:train_input_fn(batch_size))
        else:
            # TODO(typhoonzero): able to config metrics by calling tf.estimators.add_metrics()
            train_hooks = []
            if verbose == 1 and TF_VERSION_2:
                train_hooks = [PrintStatusHook("train", every_n_iter=log_every_n_iter)]
            train_spec = tf.estimator.TrainSpec(input_fn=lambda:train_input_fn(batch_size), max_steps=train_max_steps, hooks=train_hooks)
            eval_hooks = []
            if verbose == 1 and TF_VERSION_2:
                eval_hooks = [PrintStatusHook("eval", every_n_iter=log_every_n_iter)]
            eval_spec = tf.estimator.EvalSpec(input_fn=lambda:validate_input_fn(batch_size), hooks=eval_hooks, start_delay_secs=eval_start_delay_secs, throttle_secs=eval_throttle_secs)
            result = tf.estimator.train_and_evaluate(classifier, train_spec, eval_spec)
            # FIXME(typhoonzero): find out why pai will have result == None
            if not is_pai:
                print(result[0])

    print("Done training")

