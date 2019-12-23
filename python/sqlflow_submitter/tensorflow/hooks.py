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

import tensorflow as tf

# NOTE(typhoonzero): tf.estimator.LoggingTensorHook is only availble using
# Tensorflow version >= 2.0.
# PrintStatusHook will be used in tensorflow/train.py only if installed Tensorflow version >= 2.0.
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