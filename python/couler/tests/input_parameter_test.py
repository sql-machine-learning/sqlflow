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

"""
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: steps-
spec:
  # invoke the whalesay template with
  # "hello world" as the argument
  # to the message parameter
  entrypoint: hello-hello

  templates
   - name: hello-hello-hello
    # Instead of just running a container
    # This template has a sequence of steps
    steps:
    - - name: hello1            # hello1 is run before the following steps
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "hello1"
    - - name: hello2a           # double dash => run after previous step
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "hello2a"

  templates:
  - name: whalesay
    inputs:
      parameters:
      - name: message       # parameter declaration
    container:
      # run cowsay with that message input parameter as args
      image: docker/whalesay
      command: [cowsay]
      args: ["{{inputs.parameters.message}}"]
"""


from collections import OrderedDict

import couler.argo as couler
from tests.argo_yaml_test import ArgoYamlTest


def random_code():
    import random

    result = random.randint(0, 1)
    print(result)


def generate_number():
    return couler.run_script(image="python:alpine3.6", source=random_code)


def whalesay(hello_param):
    return couler.run_container(
        image="docker/whalesay", command=["cowsay"], args=[hello_param]
    )


def whalesay_two(hello_param1, hello_param2):
    return couler.run_container(
        image="docker/whalesay",
        command=["cowsay"],
        args=[hello_param1, hello_param2],
    )


class InputParametersTest(ArgoYamlTest):
    def test_input_basic(self):
        whalesay("hello1")
        inputs = couler._templates["whalesay"]["inputs"]
        expected_inputs = OrderedDict(
            [("parameters", [{"name": "para-whalesay-0"}])]
        )
        self.assertEqual(inputs, expected_inputs)

    def test_input_basic_two_calls(self):
        whalesay("hello1")
        whalesay("hello2")

        self.check_argo_yaml("input_para_golden_1.yaml")

    def test_input_basic_two_paras_2(self):
        whalesay_two("hello1", "hello2")
        whalesay_two("x", "y")

        self.check_argo_yaml("input_para_golden_2.yaml")

    def test_input_steps_1(self):
        message = "test"
        whalesay(message)
        message = generate_number()
        whalesay(message)

        self.check_argo_yaml("input_para_golden_3.yaml")
