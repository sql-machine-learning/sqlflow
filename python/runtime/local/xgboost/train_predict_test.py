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

import runtime.db as db
import runtime.testing as testing
from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.local.xgboost import pred, train


class TestXGBoostTrain(unittest.TestCase):
    def get_table_row_count(self, conn, table):
        ret = list(conn.query("SELECT COUNT(*) FROM %s" % table))
        self.assertEqual(len(ret), 1)
        ret = ret[0]
        self.assertEqual(len(ret), 1)
        return ret[0]

    def get_table_schema(self, conn, table):
        name_and_types = conn.get_table_schema(table)
        return dict(name_and_types)

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_train_and_predict(self):
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
        train_params = {"num_boost_round": 20}
        model_params = {"num_class": 3, "objective": "multi:softmax"}
        save_name = "iris.xgboost_train_model_test"
        class_name = "class"

        old_dir_name = os.getcwd()
        with tempfile.TemporaryDirectory() as tmp_dir_name:
            os.chdir(tmp_dir_name)
            eval_result = train(original_sql=original_sql,
                                model_image="sqlflow:step",
                                estimator="xgboost.gbtree",
                                datasource=ds,
                                select=select,
                                validation_select=val_select,
                                model_params=model_params,
                                train_params=train_params,
                                feature_column_map=None,
                                label_column=NumericColumn(
                                    FieldDesc(name=class_name)),
                                save=save_name)
            self.assertLess(eval_result['train']['merror'][-1], 0.01)
            self.assertLess(eval_result['validate']['merror'][-1], 0.01)

            conn = db.connect_with_data_source(ds)

            pred_select = "SELECT * FROM iris.test"
            pred(ds, pred_select, "iris.predict_result_table", class_name,
                 save_name)

            self.assertEqual(
                self.get_table_row_count(conn, "iris.test"),
                self.get_table_row_count(conn, "iris.predict_result_table"))

            schema1 = self.get_table_schema(conn, "iris.test")
            schema2 = self.get_table_schema(conn, "iris.predict_result_table")
            self.assertEqual(len(schema1), len(schema2))
            for name in schema1:
                if name == 'class':
                    self.assertEqual(schema2[name], "BIGINT")
                    continue

                self.assertTrue(name in schema2)
                self.assertEqual(schema1[name], schema2[name])

            diff_schema = schema2.keys() - schema1.keys()
            self.assertEqual(len(diff_schema), 0)

        os.chdir(old_dir_name)


if __name__ == '__main__':
    unittest.main()
