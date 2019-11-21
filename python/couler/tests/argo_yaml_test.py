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

import json
import os
import unittest

import pyaml
import yaml

import couler.argo as couler

_test_data_dir = "test_data"


class ArgoYamlTest(unittest.TestCase):
    def setUp(self):
        couler._cleanup()

    def tearDown(self):
        couler._cleanup()

    def check_argo_yaml(self, expected_fn):
        test_data_dir = os.path.join(os.path.dirname(__file__), _test_data_dir)
        with open(os.path.join(test_data_dir, expected_fn), "r") as f:
            expected = yaml.safe_load(f)
        output = yaml.safe_load(
            pyaml.dump(couler.yaml(), string_val_style="plain")
        )

        output_j = json.dumps(output, indent=2)
        expected_j = json.dumps(expected, indent=2)

        self.maxDiff = None
        self.assertEqual(output_j, expected_j)
