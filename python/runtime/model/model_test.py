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

from runtime.model import EstimatorType, Model
from runtime.tensorflow.diag import SQLFlowDiagnostic


class TestModel(unittest.TestCase):
    def test_unsupport_driver(self):
        uri = "unknown://path"
        meta = {"train_params": {"n_classes": 3}}
        m = Model(EstimatorType.XGBOOST, meta)
        with self.assertRaises(SQLFlowDiagnostic) as ctx:
            m.save(uri)
        self.assertEqual(ctx.exception.args[0],
                         "unsupported driven to save model: unknown")


if __name__ == '__main__':
    unittest.main()
