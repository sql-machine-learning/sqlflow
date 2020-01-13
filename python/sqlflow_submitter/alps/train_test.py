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
import subprocess
import unittest
from unittest import TestCase


class TestALPSTrain(TestCase):
    '''NOTE: we must test tensorflow training and predicting in separated processes, or
    TensorFlow will raise error "Graph is finalized.'''
    def test_train(self):
        if os.getenv("SQLFLOW_submitter") != "alps":
            return
        try:
            # should run this test under directory $GOPATH/sqlflow.org/sqlflow
            ret = subprocess.run([
                "/usr/local/bin/python",
                "python/sqlflow_submitter/alps/train_example.py"
            ],
                                 env={"PYTHONPATH": "python"},
                                 check=True)
            self.assertEqual(ret.returncode, 0)
        except:
            self.fail("%s" % ret.stderr)


if __name__ == '__main__':
    unittest.main()
