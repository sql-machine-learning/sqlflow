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

import couler.argo as couler


class TestArgo(unittest.TestCase):
    def test_env_list(self):
        test_d = {
            "str": "value",
            "boolean": True,
            "integer": 10,
            "decimal": 3.1415926,
            "json": '''{"key": "value"}'''
        }
        actual = couler._convert_dict_to_env_list(test_d)
        expected_list = [{
            "name": "str",
            "value": "'value'"
        }, {
            "name": "boolean",
            "value": "'True'"
        }, {
            "name": "integer",
            "value": "'10'"
        }, {
            "name": "decimal",
            "value": "'3.1415926'"
        }, {
            "name": "json",
            "value": "'{\"key\": \"value\"}'"
        }]
        self.assertEqual(actual, expected_list)


if __name__ == "__main__":
    unittest.main()
