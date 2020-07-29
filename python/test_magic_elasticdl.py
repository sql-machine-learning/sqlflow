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

import unittest

from IPython import get_ipython

ipython = get_ipython()


class TestSQLFlowMagic(unittest.TestCase):
    train_statement = """SELECT * FROM iris.train
TO TRAIN ElasticDLKerasClassifier
WITH
    model.num_classes = 10,
    train.shuffle = 120,
    train.epoch = 2,
    train.grads_to_wait = 2,
    train.tensorboard_log_dir = "",
    train.checkpoint_steps = 0,
    train.checkpoint_dir = "",
    train.keep_checkpoint_max = 0,
    eval.steps = 0,
    eval.start_delay_secs = 100,
    eval.throttle_secs = 0,
    eval.checkpoint_filename_for_init = "",
    engine.docker_image_prefix = "",
    engine.master_resource_request =
        "cpu=1,memory=4096Mi,ephemeral-storage=10240Mi",
    engine.worker_resource_request =
        "cpu=1,memory=4096Mi,ephemeral-storage=10240Mi",
    engine.minibatch_size = 10,
    engine.num_workers = 2,
    engine.volume = "",
    engine.image_pull_policy = "Always",
    engine.restart_policy = "Never",
    engine.extra_pypi_index = "",
    engine.namespace = "default",
    engine.master_pod_priority = "",
    engine.cluster_spec = "",
    engine.num_minibatches_per_task = 10,
    engine.docker_image_repository = "",
    engine.envs = ""
COLUMN
    sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO trained_elasticdl_keras_classifier;
"""

    def test_elasticdl(self):
        ipython.run_cell_magic("sqlflow", "", self.train_statement)


if __name__ == "__main__":
    unittest.main()
