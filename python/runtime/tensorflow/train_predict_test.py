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
import subprocess
import sys
import unittest
from unittest import TestCase


class TestEstimatorModels(TestCase):
    '''NOTE: we must test tensorflow training and predicting in separated
    processes, or TensorFlow will raise error "Graph is finalized.'''
    def test_estimator(self):
        try:
            # should run this test under directory $GOPATH/sqlflow.org/sqlflow
            ret = subprocess.run([
                sys.executable,
                "python/runtime/tensorflow/estimator_example.py"
            ],
                                 env=os.environ.copy(),
                                 check=True)
            self.assertEqual(ret.returncode, 0)
        except Exception as e:
            self.fail("%s" % e)

    def test_explain(self):
        try:
            # should run this test under directory $GOPATH/sqlflow.org/sqlflow
            ret = subprocess.run([
                sys.executable, "python/runtime/tensorflow/explain_example.py"
            ],
                                 env=os.environ.copy(),
                                 check=True)
            self.assertEqual(ret.returncode, 0)
        except Exception as e:
            self.fail("%s" % e)

    def test_keras(self):
        try:
            # should run this test under directory $GOPATH/sqlflow.org/sqlflow
            ret = subprocess.run(
                [sys.executable, "python/runtime/tensorflow/keras_example.py"],
                env=os.environ.copy(),
                check=True)
            self.assertEqual(ret.returncode, 0)

            ret = subprocess.run([
                sys.executable,
                "python/runtime/tensorflow/keras_example_reg.py"
            ],
                                 env=os.environ.copy(),
                                 check=True)
            self.assertEqual(ret.returncode, 0)
        except Exception as e:
            self.fail("%s" % e)


if __name__ == '__main__':
    unittest.main()
