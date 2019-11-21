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

import couler.argo as couler
from tests.argo_yaml_test import ArgoYamlTest


def producer():
    output_place = couler.artifact(path="/tmp/hello_world.txt")
    return couler.run_container(
        image="docker/whalesay:latest",
        args=["echo -n hello world > %s" % output_place.path],
        output=output_place,
    )


def consumer(message):
    couler.run_container(
        image="docker/whalesay:latest", command=["cowsay"], args=[message]
    )


class StepOutputTest(ArgoYamlTest):
    def test_producer_consumer(self):
        messge = producer()
        consumer(messge)

        self.check_argo_yaml("output_golden_1.yaml")
