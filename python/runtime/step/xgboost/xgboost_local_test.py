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
from runtime.local.submitter import submit_local_train as train
from runtime.model.model import EstimatorType
from runtime.step.create_result_table import (create_evaluate_table,
                                              create_explain_table,
                                              create_predict_table)
from runtime.step.xgboost.evaluate import evaluate
from runtime.step.xgboost.explain import explain
from runtime.step.xgboost.predict import predict


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
            eval_result = train(datasource=ds,
                                original_sql=original_sql,
                                select=select,
                                validation_select=val_select,
                                estimator_string="xgboost.gbtree",
                                model_image="sqlflow:step",
                                feature_column_map=None,
                                label_column=NumericColumn(
                                    FieldDesc(name=class_name)),
                                model_params=model_params,
                                train_params=train_params,
                                validation_params=None,
                                save=save_name,
                                load=None)

        self.assertLess(eval_result['train']['merror'][-1], 0.01)
        self.assertLess(eval_result['validate']['merror'][-1], 0.01)

        conn = db.connect_with_data_source(ds)
        pred_select = "SELECT * FROM iris.test"

        with temp_file.TemporaryDirectory(as_cwd=True):
            result_column_names, train_label_idx = create_predict_table(
                conn, select, "iris.predict_result_table",
                FieldDesc(name=class_name), "class")
            predict(ds, pred_select, "iris.predict_result_table",
                    result_column_names, train_label_idx, save_name)

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
            result_column_names = create_evaluate_table(
                conn, "iris.evaluate_result_table", ["accuracy_score"])
            evaluate(ds,
                     pred_select,
                     "iris.evaluate_result_table",
                     save_name,
                     label_name='class',
                     model_params={'validation.metrics': 'accuracy_score'},
                     result_column_names=result_column_names)

        eval_schema = self.get_table_schema(conn, "iris.evaluate_result_table")
        self.assertEqual(eval_schema.keys(), set(['loss', 'accuracy_score']))

        with temp_file.TemporaryDirectory(as_cwd=True):
            feature_column_names = [
                "petal_width", "petal_length", "sepal_width", "sepal_length"
            ]
            create_explain_table(conn, EstimatorType.XGBOOST, "TreeExplainer",
                                 "xgboost.gbtree", "iris.explain_result_table",
                                 feature_column_names)
            explain(ds, select, "TreeExplainer", {"plot_type": "decision"},
                    "iris.explain_result_table", save_name)

        explain_schema = self.get_table_schema(conn,
                                               "iris.explain_result_table")
        self.assertEqual(explain_schema.keys(), set(feature_column_names))

        with temp_file.TemporaryDirectory(as_cwd=True):
            create_explain_table(conn, EstimatorType.XGBOOST,
                                 "XGBoostExplainer", "xgboost.gbtree",
                                 "iris.explain_result_table_2",
                                 feature_column_names)
            explain(ds, select, "XGBoostExplainer", {},
                    "iris.explain_result_table_2", save_name)

        explain_schema = self.get_table_schema(conn,
                                               "iris.explain_result_table_2")
        self.assertEqual(explain_schema.keys(),
                         set(['feature', 'fscore', 'gain']))
        conn.close()


if __name__ == '__main__':
    unittest.main()
