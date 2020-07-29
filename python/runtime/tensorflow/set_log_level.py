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
from runtime.tensorflow.get_tf_version import tf_is_version2


def set_log_level(verbose, is_estimator):
    assert 0 <= verbose <= 3
    if not is_estimator and verbose == 1 or tf_is_version2():
        tf.get_logger().setLevel(
            (4 - verbose) * 10)  # logging.INFO levels range from 10~40
    elif verbose >= 2:
        tf.logging.set_verbosity(tf.logging.INFO)
