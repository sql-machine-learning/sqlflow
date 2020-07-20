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

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.model import EstimatorType, Model, load
from runtime.testing import get_datasource


class TestModel(unittest.TestCase):
    def setUp(self):
        self.cur_dir = os.getcwd()

    def tearDown(self):
        os.chdir(self.cur_dir)

    def test_save(self):
        table = "sqlflow_models.test_model"
        meta = {"train_params": {"n_classes": 3}}
        m = Model(EstimatorType.XGBOOST, meta)
        datasource = get_datasource()

        # save model
        with tempfile.TemporaryDirectory() as d:
            os.chdir(d)
            m.save(datasource, table)

        # load model
        with tempfile.TemporaryDirectory() as d:
            os.chdir(d)
            m = load(datasource, table)
            self.assertEqual(m._meta, meta)


if __name__ == '__main__':
    unittest.main()
