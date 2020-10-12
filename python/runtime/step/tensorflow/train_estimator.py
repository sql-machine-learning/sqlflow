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

import tensorflow as tf
from runtime.diagnostics import init_model, load_pretrained_model_estimator
from runtime.pai.pai_distributed import make_estimator_distributed_runconfig
from runtime.tensorflow import metrics
from runtime.tensorflow.get_tf_version import tf_is_version2
from runtime.tensorflow.train_estimator import (estimator_save,
                                                estimator_train_compiled)


def estimator_train_and_save(estimator, model_params, save, FLAGS,
                             train_dataset_fn, val_dataset_fn, train_max_steps,
                             eval_start_delay_secs, eval_throttle_secs,
                             save_checkpoints_steps, metric_names, load,
                             model_meta):
    print("Start training using estimator model...")
    is_distributed = False
    if len(FLAGS.worker_hosts.split(",")) > 1:
        is_distributed = True
    model_params["config"] = make_estimator_distributed_runconfig(
        FLAGS,
        estimator,
        is_distributed,
        save_checkpoints_steps=save_checkpoints_steps)
    ckpt_dir = FLAGS.checkpointDir if FLAGS.checkpointDir else save
    print("Using checkpoint path: %s" % ckpt_dir)
    model_params["model_dir"] = ckpt_dir

    if load:
        load_pretrained_model_estimator(estimator, model_params, load)
    classifier = init_model(estimator, model_params)

    # do not add default Accuracy metric when using estimator to train,
    # it will fail when the estimator is a regressor, and the estimator
    # will automatically add metrics. Only add additional metrics when
    # user specified with `WITH`.
    if tf_is_version2() and metric_names != ["Accuracy"]:
        classifier = tf.estimator.add_metrics(
            classifier, metrics.get_tf_metrics(metric_names))

    estimator_train_compiled(classifier, train_dataset_fn, val_dataset_fn,
                             train_max_steps, eval_start_delay_secs,
                             eval_throttle_secs)

    if FLAGS.task_index != 0:
        print("skip exporting model on worker != 0")
        return
    estimator_save(classifier, save, model_params, model_meta)
