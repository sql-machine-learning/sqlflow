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

import runtime.temp_file as temp_file
import runtime.testing as testing
from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.local import evaluate, explain, pred, train


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
        with temp_file.TemporaryDirectory(as_cwd=True):
            eval_result = train(ds, original_sql, select, val_select,
                                "xgboost.gbtree", "", None,
                                NumericColumn(FieldDesc(name="class")),
                                model_params, train_params, None,
                                "iris.xgboost_train_model_test", None)
            self.assertLess(eval_result['train']['merror'][-1], 0.01)
            self.assertLess(eval_result['validate']['merror'][-1], 0.01)

        with temp_file.TemporaryDirectory(as_cwd=True):
            pred_original_sql = """SELECT * FROM iris.test
            TO PREDICT iris.xgboost_pred_result.pred_val
            USING iris.xgboost_train_model_test;"""
            pred(ds, pred_original_sql, "SELECT * FROM iris.test",
                 "iris.xgboost_train_model_test", "pred_val", model_params,
                 "iris.xgboost_pred_result")

        with temp_file.TemporaryDirectory(as_cwd=True):
            explain_original_sql = """SELECT * FROM iris.test
            TO EXPLAIN iris.xgboost_train_model_test
            INTO iris.xgboost_explain_result;"""
            explain(ds, explain_original_sql, "SELECT * FROM iris.test",
                    "iris.xgboost_train_model_test", model_params,
                    "iris.xgboost_explain_result")

        with temp_file.TemporaryDirectory(as_cwd=True):
            evaluate_original_sql = """SELECT * FROM iris.test
            TO EVALUATE iris.xgboost_train_model_test
            WITH label_col=class
            INTO iris.xgboost_evaluate_result;"""
            evaluate(ds, evaluate_original_sql, "SELECT * FROM iris.test",
                     "class", "iris.xgboost_train_model_test", model_params,
                     "iris.xgboost_evaluate_result")


if __name__ == '__main__':
    unittest.main()
