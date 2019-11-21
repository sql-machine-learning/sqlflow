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


class ResourceTest(ArgoYamlTest):
    def test_resource_setup(self):
        resources = {"cpu": "1", "memory": "100Mi"}

        couler.run_container(
            image="docker/whalesay",
            command=["cowsay"],
            args=["resource test"],
            resources=resources,
        )
        # Because test environment between local and CI is different,
        # we can not compare the YAML directly.
        _test_data_dir = "test_data"
        test_data_dir = os.path.join(os.path.dirname(__file__), _test_data_dir)
        with open(
            os.path.join(test_data_dir, "resource_config_golden.yaml"), "r"
        ) as f:
            expected = yaml.safe_load(f)
        output = yaml.safe_load(
            pyaml.dump(couler.yaml(), string_val_style="plain")
        )
        _resources = output["spec"]["templates"][1]["container"]["resources"]
        _expected_resources = expected["spec"]["templates"][1]["container"][
            "resources"
        ]

        self.assertEqual(_resources, _expected_resources)
        couler._cleanup()
