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

import os
import tempfile
import unittest

from runtime.model.tar import unzip_dir, zip_dir


class TestTarOperator(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.TemporaryDirectory()
        self.cur_dir = os.getcwd()
        os.chdir(self.test_dir.name)

    def tearDown(self):
        self.test_dir.cleanup()
        os.chdir(self.cur_dir)

    def test_tar(self):
        # create the test file tree:
        #
        # |-sqlflow_tar
        #   |-sqlflow_sub_dir
        #     |-hello.py
        test_dir = "sqlflow_tar"
        test_sub_dir = "sqlflow_sub_dir"
        test_py_file = "hello.py"
        test_py_content = "print('hello SQLFlow!')"

        fullpath = os.path.join(test_dir, test_sub_dir)
        os.makedirs(fullpath)
        with open(os.path.join(fullpath, test_py_file), "w") as f:
            f.write(test_py_content)

        zip_dir(fullpath, "sqlflow.tar.gz")
        unzip_dir("sqlflow.tar.gz", "output")
        self.assertTrue(os.path.isdir("output/sqlflow_tar/sqlflow_sub_dir"))
        self.assertTrue(
            os.path.isfile("output/sqlflow_tar/sqlflow_sub_dir/hello.py"))
        with open(os.path.join(fullpath, test_py_file), "r") as f:
            self.assertEqual(f.read(), test_py_content)


if __name__ == '__main__':
    unittest.main()
