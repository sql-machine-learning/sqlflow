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

import base64
import unittest

from couler.pyfunc import (
    _argo_safe_name,
    body,
    encode_base64,
    invocation_location,
    workflow_filename,
)


class PyfuncTest(unittest.TestCase):
    def test_argo_safe_name(self):
        self.assertIsNone(_argo_safe_name(None))
        self.assertEqual(_argo_safe_name("a_b"), "a-b")
        self.assertEqual(_argo_safe_name("a.b"), "a-b")
        self.assertEqual(_argo_safe_name("a_.b"), "a--b")
        self.assertEqual(_argo_safe_name("_abc."), "-abc-")

    def test_body(self):
        # Check None
        self.assertIsNone(body(None))
        # A real function
        code = """
func_name = workflow_filename()
# Here we assume using pytest to trigger the unit tests
self.assertEqual(func_name, "pytest")
"""
        self.assertEqual(code, body(self.test_get_root_caller_filename))

    def test_get_root_caller_filename(self):
        func_name = workflow_filename()
        # Here we assume using pytest to trigger the unit tests
        self.assertEqual(func_name, "pytest")

    def test_invocation_location(self):
        def inner_func():
            func_name, _ = invocation_location()
            self.assertEqual("test-invocation-location", func_name)

        inner_func()

    def test_encode_base64(self):
        s = "test encode string"
        encode = encode_base64(s)
        decode = str(base64.b64decode(encode), "utf-8")
        self.assertEqual(s, decode)
