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

import runtime.db as db
import runtime.temp_file as temp_file
import runtime.testing as testing
from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.local.xgboost_submitter.evaluate import evaluate
from runtime.local.xgboost_submitter.explain import explain
from runtime.local.xgboost_submitter.predict import pred
from runtime.local.xgboost_submitter.train import train


class TestXGBoostTrain(unittest.TestCase):
    def get_table_row_count(self, conn, table):
        rs = conn.query("SELECT COUNT(*) FROM %s" % table)
        ret = list(rs)
        rs.close()
        self.assertEqual(len(ret), 1)
        ret = ret[0]
        self.assertEqual(len(ret), 1)
        return ret[0]

    def get_table_schema(self, conn, table):
        name_and_types = conn.get_table_schema(table)
        return dict(name_and_types)

    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_main(self):
        ds = testing.get_datasource()
        original_sql = """SELECT * FROM iris.train
        TO TRAIN xgboost.gbtree
        WITH
            objective="multi:softprob",
            num_boost_round=20,
            num_class=3,
            validation.select="SELECT * FROM iris.test"
        LABEL class
        INTO iris.xgboost_train_model_test;
        """

        select = "SELECT * FROM iris.train"
        val_select = "SELECT * FROM iris.test"
        train_params = {"num_boost_round": 20}
        model_params = {"num_class": 3, "objective": "multi:softprob"}
        save_name = "iris.xgboost_train_model_test"
        class_name = "class"

        with temp_file.TemporaryDirectory(as_cwd=True):
            eval_result = train(original_sql=original_sql,
                                model_image="sqlflow:step",
                                estimator_string="xgboost.gbtree",
                                datasource=ds,
                                select=select,
                                validation_select=val_select,
                                model_params=model_params,
                                train_params=train_params,
                                validation_params=None,
                                feature_column_map=None,
                                label_column=NumericColumn(
                                    FieldDesc(name=class_name)),
                                save=save_name)

        self.assertLess(eval_result['train']['merror'][-1], 0.01)
        self.assertLess(eval_result['validate']['merror'][-1], 0.01)

        conn = db.connect_with_data_source(ds)
        pred_select = "SELECT * FROM iris.test"

        with temp_file.TemporaryDirectory(as_cwd=True):
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

        with temp_file.TemporaryDirectory(as_cwd=True):
            evaluate(ds, pred_select, "iris.evaluate_result_table", save_name,
                     'class', {'validation.metrics': 'accuracy_score'})

        eval_schema = self.get_table_schema(conn, "iris.evaluate_result_table")
        self.assertEqual(eval_schema.keys(), set(['loss', 'accuracy_score']))

        with temp_file.TemporaryDirectory(as_cwd=True):
            explain(ds, select, "TreeExplainer", {"plot_type": "decision"},
                    "iris.explain_result_table", save_name)

        explain_schema = self.get_table_schema(conn,
                                               "iris.explain_result_table")
        self.assertEqual(
            explain_schema.keys(),
            set(["petal_width", "petal_length", "sepal_width",
                 "sepal_length"]))

        with temp_file.TemporaryDirectory(as_cwd=True):
            explain(ds, select, "XGBoostExplainer", {},
                    "iris.explain_result_table_2", save_name)

        explain_schema = self.get_table_schema(conn,
                                               "iris.explain_result_table_2")
        self.assertEqual(explain_schema.keys(),
                         set(['feature', 'fscore', 'gain']))
        conn.close()


if __name__ == '__main__':
    unittest.main()
