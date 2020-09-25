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

import multiprocessing
import unittest

import runtime.db as db
import runtime.temp_file as temp_file
import runtime.testing as testing
from runtime.feature.column import NumericColumn
from runtime.feature.field_desc import FieldDesc
from runtime.local.tensorflow_submitter.evaluate import evaluate
from runtime.local.tensorflow_submitter.explain import explain
from runtime.local.tensorflow_submitter.predict import pred
from runtime.local.tensorflow_submitter.train import train


class TestTensorFlowLocalSubmitter(unittest.TestCase):
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

    # NOTE(sneaxiy): do not know why the xgboost explain ut would
    # be conflict with the TensorFlow explain ut. The error is
    # "summary.png" not found. I try to fix the ut by using
    # another process to run them.
    def check_main(self, estimator):
        p = multiprocessing.Process(target=self.check_main_impl,
                                    args=(estimator, ))
        p.start()
        p.join()
        self.assertEqual(p.exitcode, 0)

    def check_main_impl(self, estimator):
        if testing.get_driver() != "mysql":
            return

        ds = testing.get_datasource()
        original_sql = """SELECT * FROM iris.train
        TO TRAIN %s
        WITH
            model.hidden_units=[32,64],
            model.n_classes=3,
            validation.select="SELECT * FROM iris.test"
        LABEL class
        INTO iris.tensorflow_train_model_test;
        """ % estimator

        select = "SELECT * FROM iris.train"
        val_select = "SELECT * FROM iris.test"
        train_params = {"batch_size": 10}
        model_params = {"n_classes": 3, "hidden_units": [32, 64]}
        save_name = "iris.tensorflow_train_model_test"
        class_name = "class"

        with temp_file.TemporaryDirectory(as_cwd=True):
            train(original_sql=original_sql,
                  model_image="sqlflow:step",
                  estimator_string=estimator,
                  datasource=ds,
                  select=select,
                  validation_select=val_select,
                  model_params=model_params,
                  train_params=train_params,
                  validation_params=None,
                  feature_column_map=None,
                  label_column=NumericColumn(
                      FieldDesc(name=class_name, shape=[])),
                  save=save_name)

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
            evaluate(ds, select, "iris.evaluate_result_table", save_name,
                     class_name, {'validation.metrics': 'Accuracy'})

        eval_schema = self.get_table_schema(conn, "iris.evaluate_result_table")
        eval_schema = set([k.lower() for k in eval_schema.keys()])
        self.assertEqual(eval_schema, set(['loss', 'accuracy']))

        with temp_file.TemporaryDirectory(as_cwd=True):
            explain(ds, select, None, {"plot_type": "bar"},
                    "iris.explain_result_table", save_name)

        explain_schema = self.get_table_schema(conn,
                                               "iris.explain_result_table")
        self.assertEqual(
            explain_schema.keys(),
            set(['petal_length', 'petal_width', 'sepal_length',
                 'sepal_width']))
        conn.close()
