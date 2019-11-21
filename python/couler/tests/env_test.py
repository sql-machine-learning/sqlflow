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

import contextlib
import unittest
from io import StringIO

import couler.argo as couler

echo_argo_yaml = """
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: pytest-
spec:
  entrypoint: pytest
  templates:
    - name: pytest
      steps:
        - - name: echo-46
            template: echo
    - name: echo
      container:
        image: alpine:3.6
        command:
          - bash
          - -c
          - 'echo ${MESSAGE}'
        env:
          - name: MESSAGE
            value: 'Hello World!'
"""


class ArgoUnitTest(unittest.TestCase):
    def setUp(self):
        couler._cleanup()

    def test_dump_yaml(self):
        temp_stdout = StringIO()

        def echo():
            return couler.run_container(
                image="alpine:3.6",
                env={"MESSAGE": "Hello World!"},
                command=["""echo ${MESSAGE}"""],
            )

        echo()
        with contextlib.redirect_stdout(temp_stdout):
            couler._dump_yaml()
        output = temp_stdout.getvalue().strip()
        assert output == echo_argo_yaml.strip()
