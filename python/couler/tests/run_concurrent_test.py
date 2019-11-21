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

import pyaml
import yaml

import couler.argo as couler
from tests.argo_yaml_test import ArgoYamlTest


def whalesay(hello_param):
    return couler.run_container(
        image="docker/whalesay", command=["cowsay"], args=[hello_param]
    )


def heads():
    return couler.run_container(
        image="docker/whalesay", command='echo "it was heads"'
    )


def tails():
    return couler.run_container(
        image="docker/whalesay", command='echo "it was tails"'
    )


class ConcurrentTest(ArgoYamlTest):
    def setUp(self):
        couler._cleanup()

    def test_run_concurrent(self):
        couler.concurrent(
            [lambda: whalesay("hello1"), lambda: heads(), lambda: tails()]
        )

        _test_data_dir = "test_data"
        test_data_dir = os.path.join(os.path.dirname(__file__), _test_data_dir)
        with open(
            os.path.join(test_data_dir, "run_concurrent_golden.yaml"), "r"
        ) as f:
            expected = yaml.safe_load(f)
        output = yaml.safe_load(
            pyaml.dump(couler.yaml(), string_val_style="plain")
        )
        # Because test environment between local and CI is different,
        # we can not compare the YAML directly.
        steps = output["spec"]["templates"][0]["steps"][0]
        expected_steps = expected["spec"]["templates"][0]["steps"][0]

        self.assertEqual(len(steps), len(expected_steps))
        for index in range(len(steps)):
            _step = steps[index]
            _expected_step = expected_steps[index]
            self.assertEqual(_step["template"], _expected_step["template"])

        couler._cleanup()
