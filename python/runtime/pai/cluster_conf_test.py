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
from unittest import TestCase

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai.cluster_conf import get_cluster_config


class ClusterConfigTest(TestCase):
    def test_get_cluster_config(self):
        attrs = {
            "train.worker_cpu": 100,
            "train.worker_gpu": 0,
            "train.ps_cpu": 100,
        }
        conf = get_cluster_config(attrs)
        self.assertEqual(100, conf["worker"]["cpu"])
        self.assertEqual(0, conf["worker"]["gpu"])
        self.assertEqual(100, conf["ps"]["cpu"])

        attrs["train.worker_cpu"] = 100.0
        with self.assertRaises(SQLFlowDiagnostic) as ctx:
            get_cluster_config(attrs)
        self.assertEqual("value for cluster config should be int",
                         ctx.exception.args[0])

        attrs["train.worker_cpu"] = 100
        attrs["train.num_evaluator"] = 2
        with self.assertRaises(SQLFlowDiagnostic) as ctx:
            get_cluster_config(attrs)
        self.assertEqual("train.num_evaluator should only be 1 or 0",
                         ctx.exception.args[0])


if __name__ == "__main__":
    unittest.main()
