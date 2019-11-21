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
apiVersion: v1
data:
  passwd: ZGVm
  uname: YWJj
kind: Secret
metadata:
  name: couler-secret-secret-59
type: Opaque

---
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: pytest-
spec:
  entrypoint: pytest
  templates:
    - name: pytest
      steps:
        - - name: whalesay-68
            template: whalesay
    - name: whalesay
      container:
        image: python:alpine3.6
        command:
          - bash
          - -c
          - 'echo $uname'
        env:
          - name: uname
            valueFrom:
              secretKeyRef:
                key: uname
                name: couler-secret-secret-59
          - name: passwd
            valueFrom:
              secretKeyRef:
                key: passwd
                name: couler-secret-secret-59
"""


class SecretTest(unittest.TestCase):
    def setUp(self):
        couler._cleanup()

    def test_dump_yaml(self):
        temp_stdout = StringIO()

        def echo():
            user_info = {"uname": "abc", "passwd": "def"}
            secret_info = couler.secret(user_info)

            def whalesay(secret):
                couler.run_container(
                    image="python:alpine3.6",
                    secret=secret_info,
                    command="echo $uname",
                )

            whalesay(secret_info)

        echo()
        with contextlib.redirect_stdout(temp_stdout):
            couler._dump_yaml()
        output = temp_stdout.getvalue().strip()
        self.assertEqual(output, echo_argo_yaml.strip())
