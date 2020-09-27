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

import json
import os
import unittest
from unittest import TestCase

import runtime.feature.column as fc
import runtime.feature.field_desc as fd
import runtime.testing as testing
import runtime.xgboost as xgboost_extended  # noqa: F401
import tensorflow as tf  # noqa: E0401,F401
from runtime.pai import (evaluate, explain, get_pai_tf_cmd, pai_model, predict,
                         train)
from runtime.pai.cluster_conf import get_cluster_config
from runtime.pai.pai_distributed import define_tf_flags


class SubmitterTestCase(TestCase):
    def test_get_oss_model_url(self):
        url = pai_model.get_oss_model_url("user_a/model")
        self.assertEqual("oss://sqlflow-models/user_a/model", url)

    def test_get_pai_tf_cmd(self):
        conf = get_cluster_config({})
        os.environ["SQLFLOW_OSS_CHECKPOINT_CONFIG"] = \
            '\'\'\'{"arn":"arn", "host":"host"}\'\'\''
        cmd = get_pai_tf_cmd.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev "
            "-DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz "
            "-DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,"
            "odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table "
            "-DhyperParameters='params.txt' "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?"
            "role_arn=arn/pai2osstestproject&host=host' "
            "-DgpuRequired='0'")
        self.assertEqual(expected, cmd)

        conf = get_cluster_config({"train.num_workers": 5})
        cmd = get_pai_tf_cmd.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev "
            "-DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz "
            "-DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,"
            "odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table "
            "-DhyperParameters='params.txt' "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?"
            "role_arn=arn/pai2osstestproject&host=host' "
            r'''-Dcluster="{\"ps\": {\"count\": 1, \"cpu\": 200, \"gpu\": 0}'''
            r''', \"worker\": {\"count\": 5, \"cpu\": 400, \"gpu\": 0}}"''')
        self.assertEqual(expected, cmd)
        del os.environ["SQLFLOW_OSS_CHECKPOINT_CONFIG"]


iris_feature_column_names = [
    "sepal_length",
    "sepal_width",
    "petal_length",
    "petal_width",
]

feature_column_map = {
    "feature_columns": [fc.NumericColumn(fd.FieldDesc(name="sepal_length"))]
}
label_column = fc.NumericColumn(fd.FieldDesc(name="class"))


@unittest.skipUnless(testing.get_driver() == "maxcompute"
                     and testing.get_submitter() == "pai",
                     "skip non PAI tests")
class SubmitPAITrainTask(TestCase):
    def test_submit_pai_train_task(self):
        model_params = dict()
        model_params["hidden_units"] = [10, 20]
        model_params["n_classes"] = 3

        original_sql = """
SELECT * FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO e2etest_pai_dnn;"""

        train(testing.get_datasource(), original_sql,
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train", "",
              "DNNClassifier", "", feature_column_map, label_column,
              model_params, {}, "e2etest_pai_dnn", None)

    def test_submit_pai_predict_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO PREDICT alifin_jtest_dev.pai_dnn_predict.class
USING e2etest_pai_dnn;"""
        predict(testing.get_datasource(), original_sql,
                """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test""",
                "e2etest_pai_dnn", "class", {},
                "alifin_jtest_dev.pai_dnn_predict")

    def test_submit_pai_explain_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO EXPLAIN e2etest_pai_dnn
WITH label_col=class
INTO alifin_jtest_dev.pai_dnn_explain_result;"""
        explain(testing.get_datasource(), original_sql,
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "e2etest_pai_dnn", {"label_col": "class"},
                "alifin_jtest_dev.pai_dnn_explain_result")

    def test_submit_pai_tf_evaluate_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO EXPLAIN e2etest_pai_dnn
WITH label_col=class
INTO alifin_jtest_dev.pai_dnn_explain_result;"""
        evaluate(testing.get_datasource(), original_sql,
                 "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                 "e2etest_pai_dnn", {"validation.metrics": "Accuracy,Recall"},
                 "alifin_jtest_dev.e2etest_pai_dnn_evaluate_result")

    def test_submit_xgb_train_task(self):
        original_sql = """SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH objective="multi:softprob", num_class=3, eta=0.4, booster="gbtree"
     validatioin.select="select * from alifin_jtest_dev.sqlflow_iris_test"
LABEL class
INTO e2etest_xgb_classify_model;"""
        model_params = {
            "eta": 0.4,
            "num_class": 3,
            "objective": "multi:softprob"
        }
        train_params = {"num_boost_round": 10}
        train(testing.get_datasource(), original_sql,
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
              "xgboost.gbtree", "", feature_column_map, label_column,
              model_params, train_params, "e2etest_xgb_classify_model", None)

    def test_submit_pai_xgb_predict_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO PREDICT alifin_jtest_dev.pai_xgb_predict.class
USING e2etest_xgb_classify_model;"""
        predict(testing.get_datasource(), original_sql,
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "e2etest_xgb_classify_model", "class", {},
                "alifin_jtest_dev.pai_xgb_predict")

    def test_submit_pai_xgb_explain_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO EXPLAIN e2etest_xgb_classify_model
WITH label_col=class
INTO alifin_jtest_dev.e2etest_xgb_explain_result;"""
        explain(testing.get_datasource(), original_sql,
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                "e2etest_xgb_classify_model", {"label_col": "class"},
                "alifin_jtest_dev.e2etest_xgb_explain_result")

    def test_submit_pai_xgb_evaluate_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO EVALUATE e2etest_xgb_classify_model
WITH validation.metrics=accuracy_score
INTO alifin_jtest_dev.e2etest_pai_xgb_evaluate_result;"""
        evaluate(testing.get_datasource(), original_sql,
                 "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                 "e2etest_xgb_classify_model",
                 {"validation.metrics": "accuracy_score"},
                 "alifin_jtest_dev.e2etest_pai_xgb_evaluate_result")

    def test_submit_pai_kmeans_train_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_train
TO TRAIN KMeans
WITH model.excluded_columns="class",
     model.idx_table_name="alifin_jtest_dev.e2e_test_kmeans_output_idx"
INTO e2e_test_kmeans;"""

        train(
            testing.get_datasource(), original_sql,
            "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train", "", "KMeans",
            "", feature_column_map, None, {
                "excluded_columns": "class",
                "idx_table_name": "alifin_jtest_dev.e2e_test_kmeans_output_idx"
            }, {"feature_column_names": iris_feature_column_names},
            "e2e_test_kmeans", None)

    def test_submit_pai_random_forest_train_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_train
TO TRAIN RandomForests
WITH model.tree_num=3
LABEL class
INTO e2e_test_random_forest;"""
        train(testing.get_datasource(), original_sql,
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train", "",
              "RandomForests", "", feature_column_map, label_column,
              {"tree_num": 3}, {
                  "feature_column_names":
                  iris_feature_column_names,
                  "label_meta":
                  json.loads(label_column.get_field_desc()[0].to_json())
              }, "e2e_test_random_forest_wuyi", None)

    def test_submit_pai_random_forest_predict_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test
TO PREDICT alifin_jtest_dev.pai_rf_predict.class
USING e2e_test_random_forest_wuyi;"""
        predict(testing.get_datasource(), original_sql,
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "e2e_test_random_forest_wuyi", "class", {},
                "alifin_jtest_dev.pai_rf_predict")

    def test_submit_pai_random_forest_explain_task(self):
        original_sql = """SELECT * FROM alifin_jtest_dev.sqlflow_iris_train
TO EXPLAIN e2e_test_random_forest_wuyi
WITH label_col=class
INTO alifin_jtest_dev.e2etest_random_forest_explain_result;"""
        explain(testing.get_datasource(), original_sql,
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                "e2e_test_random_forest_wuyi", {"label_col": "class"},
                "alifin_jtest_dev.e2etest_random_forest_explain_result")


# NOTE(typhoonzero): below tests must run under PAI Docker environment.
@unittest.skipUnless(testing.get_driver() == "maxcompute"
                     and testing.get_submitter() == "pai",
                     "skip non PAI tests")
class LocalRunPAITrainTask(TestCase):
    def test_pai_train_step(self):
        from runtime.step.tensorflow.train import train_step
        model_params = dict()
        model_params["hidden_units"] = [10, 20]
        model_params["n_classes"] = 3

        original_sql = """
SELECT * FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO e2etest_pai_dnn;"""
        datasource = testing.get_datasource()
        save = "e2etest_pai_dnn"

        FLAGS = define_tf_flags()
        FLAGS.sqlflow_oss_ak = os.getenv("SQLFLOW_OSS_AK")
        FLAGS.sqlflow_oss_sk = os.getenv("SQLFLOW_OSS_SK")
        FLAGS.sqlflow_oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")

        oss_path_to_save = pai_model.get_oss_model_save_path(datasource,
                                                             save,
                                                             user="")
        FLAGS.sqlflow_oss_modeldir = pai_model.get_oss_model_url(
            oss_path_to_save)

        train_step(original_sql, "", "DNNClassifier", datasource,
                   "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train", "",
                   "alifin_jtest_dev.sqlflow_iris_train", "", model_params, {},
                   feature_column_map, label_column, save, None)


if __name__ == "__main__":
    unittest.main()
