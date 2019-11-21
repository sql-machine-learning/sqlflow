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

import unittest

import couler.argo as couler


class ArgoTest(unittest.TestCase):
    def setUp(self):
        couler._cleanup()

    def test_run_script(self):
        # Source is None
        with self.assertRaises(ValueError):
            couler.run_script("image1", command="python")

        # A bash script
        self.assertEqual(len(couler._templates), 0)
        couler.run_script("image1", command="bash", source="ls")
        self.assertEqual(len(couler._templates), 1)
        template = couler._templates["test-run-script"]
        self.assertEqual("test-run-script", template["name"])
        self._verify_script_body(
            template["script"],
            image="image1",
            command=["bash"],
            source="ls",
            env=None,
        )
        couler._cleanup()

        # A python script
        self.assertEqual(len(couler._templates), 0)
        couler.run_script("image1", command="python", source=self.setUp)
        self.assertEqual(len(couler._templates), 1)
        template = couler._templates["test-run-script"]
        self.assertEqual("test-run-script", template["name"])
        self._verify_script_body(
            template["script"],
            image="image1",
            command=["python"],
            source="\ncouler._cleanup()\n",
            env=None,
        )
        couler._cleanup()

        # Command is not specified, should use python
        self.assertEqual(len(couler._templates), 0)
        couler.run_script("image1", source=self.setUp)
        self.assertEqual(len(couler._templates), 1)
        template = couler._templates["test-run-script"]
        self.assertEqual("test-run-script", template["name"])
        self._verify_script_body(
            template["script"],
            image="image1",
            command=["python"],
            source="\ncouler._cleanup()\n",
            env=None,
        )
        couler._cleanup()

    def test_create_job(self):
        success_condition = "status.succeeded > 0"
        failure_condition = "status.failed > 3"
        # Null manifest
        with self.assertRaises(ValueError):
            couler._create_job(
                manifest=None,
                action="create",
                success_condition=success_condition,
                failure_condition=failure_condition,
            )
        # Have a manifest
        manifest = """
        apiVersion: batch/v1
        kind: Job
        metadata:
          generateName: rand-num-
        spec:
          template:
            spec:
              containers:
              - name: rand
                image: python:alpine3.6
                command: ["python random_num.py"]
        """
        resource = couler._create_job(
            manifest=manifest,
            action="create",
            success_condition=success_condition,
            failure_condition=failure_condition,
        )
        self.assertEqual(resource["action"], "create")
        self.assertEqual(resource["setOwnerReference"], "true")
        self.assertEqual(resource["successCondition"], success_condition)
        self.assertEqual(resource["failureCondition"], failure_condition)
        self.assertEqual(resource["manifest"], manifest)

    def _verify_script_body(
        self, script_to_check, image, command, source, env
    ):
        self.assertEqual(script_to_check.get("image", None), image)
        self.assertEqual(script_to_check.get("command", None), command)
        self.assertEqual(script_to_check.get("source", None), source)
        self.assertEqual(script_to_check.get("env", None), env)
