# -*- coding: utf-8 -*-
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
# limitations under the License

import unittest

from runtime import testing
from runtime.dbapi.pyalisa.pop import Pop


@unittest.skipUnless(testing.get_driver() == "alisa", "Skip non-alisa test")
class TestPop(unittest.TestCase):
    def test_signature(self):
        params = {
            "name": "由由",
            "age": "3",
            "homepage": "http://little4.kg?true"
        }
        sign = Pop.signature(params, "POST", "test_secret_key")
        self.assertEqual("6kvgvUDEHtFdZKj8+HhtAS1ovHY=", sign)


if __name__ == "__main__":
    unittest.main()
