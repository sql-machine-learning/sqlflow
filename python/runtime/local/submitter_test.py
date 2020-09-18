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

import runtime.testing as testing
from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.local import train


class TestXGBoostTrain(unittest.TestCase):
    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_train(self):
        ds = testing.get_datasource()
        original_sql = """SELECT * FROM iris.train
        TO TRAIN xgboost.gbtree
        WITH
            objective="multi:softmax",
            num_boost_round=20,
            num_class=3,
            validation.select="SELECT * FROM iris.test"
        INTO iris.xgboost_train_model_test;
        """
        select = "SELECT * FROM iris.train"
        val_select = "SELECT * FROM iris.test"
        train_params = {
            "num_boost_round": 20,
        }
        model_params = {"num_class": 3, "objective": "multi:softmax"}
        eval_result = train(ds, original_sql, select, val_select,
                            "xgboost.gbtree", "", None,
                            NumericColumn(FieldDesc(name="class")),
                            model_params, train_params, None,
                            "iris.xgboost_train_model_test", None)
        self.assertLess(eval_result['train']['merror'][-1], 0.01)
        self.assertLess(eval_result['validate']['merror'][-1], 0.01)


if __name__ == '__main__':
    unittest.main()
