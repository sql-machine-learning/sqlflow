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

from runtime.pai import pai_model
from runtime.pai.entry import entrypoint
from runtime.pai.pai_distributed import define_tf_flags, set_oss_environs


def init_pai_local_tf_flags_and_envs(oss_model_dir):
    FLAGS = define_tf_flags()
    FLAGS.sqlflow_oss_ak = os.getenv("SQLFLOW_OSS_AK")
    FLAGS.sqlflow_oss_sk = os.getenv("SQLFLOW_OSS_SK")
    FLAGS.sqlflow_oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
    if not oss_model_dir.startswith("oss://"):
        oss_model_dir = pai_model.get_oss_model_url(oss_model_dir)
    FLAGS.sqlflow_oss_modeldir = oss_model_dir
    FLAGS.checkpointDir = os.getcwd()
    set_oss_environs(FLAGS)


def try_pai_local_run(params, oss_model_dir):
    if os.getenv("SQLFLOW_submitter") == "pai_local":
        init_pai_local_tf_flags_and_envs(oss_model_dir)
        print('start to run using pai_local submitter ...')
        entrypoint(params)
        return True
    else:
        return False
