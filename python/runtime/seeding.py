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


def get_tf_random_seed():
    """
    get_tf_random_seed returns an integer from the environment variable
    SQLFLOW_TF_RANDOM_SEED, that can be used as a random seed.
    Args:
        None
    Return:
        int or None
    """
    env = os.environ.get('SQLFLOW_TF_RANDOM_SEED', None)
    return int(env) if env is not None else None
