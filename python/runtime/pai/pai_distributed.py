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

from __future__ import absolute_import

import json
import os

from runtime.seeding import get_tf_random_seed

try:
    import tensorflow.compat.v1 as tf
except:  # noqa: E722
    import tensorflow as tf

# This module contain utilities for PAI distributed training.
# Note that currently PAI only support TensorFlow 1.x versions
# importing this module should make sure that currently installed
# tensorflow is version 1.x.


class DefaultFlags(object):
    def __init__(self):
        self.task_index = 0
        self.ps_hosts = ""
        self.worker_hosts = ""
        self.job_name = ""
        self.checkpointDir = ""
        self.tables = ""
        self.outputs = ""
        self.sqlflow_oss_ak = ""
        self.sqlflow_oss_sk = ""
        self.sqlflow_oss_ep = ""
        self.sqlflow_oss_modeldir = ""


def define_tf_flags():
    if os.environ.get("SQLFLOW_USE_DEFAULT_FLAGS", "").lower() == "true":
        return DefaultFlags()

    # NOTE: make sure that tf.app.flags.FLAGS is only defined once
    if hasattr(tf.app.flags.FLAGS, "task_index"):
        return tf.app.flags.FLAGS

    tf.app.flags.DEFINE_integer("task_index", 0, "Worker task index")
    tf.app.flags.DEFINE_string("ps_hosts", "", "ps hosts")
    tf.app.flags.DEFINE_string("worker_hosts", "", "worker hosts")
    tf.app.flags.DEFINE_string("job_name", 'worker', "job name: worker or ps")
    tf.app.flags.DEFINE_string("checkpointDir", "", "oss info")
    tf.app.flags.DEFINE_string("tables", "", "required by PAI-TF 1.15")
    tf.app.flags.DEFINE_string("outputs", "", "required by PAI-TF 1.15")

    tf.app.flags.DEFINE_string("sqlflow_oss_ak", "",
                               "oss ak, for writing saved models")
    tf.app.flags.DEFINE_string("sqlflow_oss_sk", "",
                               "oss sk, for writing saved models")
    tf.app.flags.DEFINE_string("sqlflow_oss_ep", "",
                               "oss endpoint, for writing saved models")
    tf.app.flags.DEFINE_string("sqlflow_oss_modeldir", "",
                               "oss model dir, where the model will be saved")

    return tf.app.flags.FLAGS


def set_oss_environs(FLAGS):
    # set OSS credentials env from pai flags for later model saving
    os.environ["SQLFLOW_OSS_AK"] = FLAGS.sqlflow_oss_ak
    os.environ["SQLFLOW_OSS_SK"] = FLAGS.sqlflow_oss_sk
    os.environ["SQLFLOW_OSS_MODEL_ENDPOINT"] = FLAGS.sqlflow_oss_ep


# make_distributed_info_without_evaluator and dump_into_tf_config are used to
# dump distributed configurations into environment variable TF_CONFIG so that
# TensorFlow can recognize it.
def make_distributed_info_without_evaluator(FLAGS):
    worker_hosts = FLAGS.worker_hosts.split(",")
    ps_hosts = FLAGS.ps_hosts.split(",")
    if len(worker_hosts) > 1:
        cluster = {
            "chief": [worker_hosts[0]],
            "worker": worker_hosts[1:],
            "ps": ps_hosts
        }
    else:
        cluster = {"chief": [worker_hosts[0]], "ps": ps_hosts}

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
    os.environ['TF_CONFIG'] = json.dumps({
        'cluster': cluster,
        'task': {
            'type': task_type,
            'index': task_index
        }
    })


def make_estimator_distributed_runconfig(FLAGS,
                                         estimator,
                                         is_distributed,
                                         save_checkpoints_steps=100):
    if is_distributed:
        cluster, task_type, task_index = make_distributed_info_without_evaluator(  # noqa: E501
            FLAGS)
        dump_into_tf_config(cluster, task_type, task_index)
        device_filters = None
        if estimator in (tf.estimator.BoostedTreesClassifier,
                         tf.estimator.BoostedTreesRegressor):
            # TFBT doesn't work with tf.contrib.distribute at the moment.
            # Use estimator distributed training instead, see
            # https://github.com/tensorflow/tensorflow/issues/32081
            dist_strategy = None
            if task_type != 'ps':
                # Disable communication between workers, see
                # https://github.com/tensorflow/tensorflow/issues/21745
                device_filters = [
                    '/job:ps',
                    '/job:%s/task:%d' % (task_type, task_index)
                ]
        else:
            dist_strategy = tf.distribute.experimental.ParameterServerStrategy(
            )
        run_config = tf.estimator.RunConfig(
            tf_random_seed=get_tf_random_seed(),
            save_checkpoints_steps=save_checkpoints_steps,
            train_distribute=dist_strategy,
            session_config=tf.ConfigProto(log_device_placement=True,
                                          device_filters=device_filters))
    else:
        run_config = tf.estimator.RunConfig(
            tf_random_seed=get_tf_random_seed(),
            save_checkpoints_steps=save_checkpoints_steps)
    return run_config
