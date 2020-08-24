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
import unittest
from unittest import TestCase

import runtime.testing as testing
import runtime.xgboost as xgboost_extended  # noqa: F401
import tensorflow as tf  # noqa: F401
from runtime.pai import (evaluate, explain, get_pai_tf_cmd, pai_model, predict,
                         train)
from runtime.pai.cluster_conf import get_cluster_config


class SubmitterTestCase(TestCase):
    def test_get_oss_model_url(self):
        url = pai_model.get_oss_model_url("user_a/model")
        self.assertEqual("oss://sqlflow-models/user_a/model", url)

    def test_get_pai_tf_cmd(self):
        conf = get_cluster_config({})
        os.environ[
            "SQLFLOW_OSS_CHECKPOINT_CONFIG"] = '{"arn":"arn", "host":"host"}'
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

iris_feature_column_names_map = dict()
iris_feature_column_names_map["feature_columns"] = [
    "sepal_length",
    "sepal_width",
    "petal_length",
    "petal_width",
]

iris_feature_metas = dict()
iris_feature_metas["sepal_length"] = {
    "feature_name": "sepal_length",
    "dtype": "float32",
    "delimiter": "",
    "format": "",
    "shape": [1],
    "is_sparse": "false" == "true"
}
iris_feature_metas["sepal_width"] = {
    "feature_name": "sepal_width",
    "dtype": "float32",
    "delimiter": "",
    "format": "",
    "shape": [1],
    "is_sparse": "false" == "true"
}
iris_feature_metas["petal_length"] = {
    "feature_name": "petal_length",
    "dtype": "float32",
    "delimiter": "",
    "format": "",
    "shape": [1],
    "is_sparse": "false" == "true"
}
iris_feature_metas["petal_width"] = {
    "feature_name": "petal_width",
    "dtype": "float32",
    "delimiter": "",
    "format": "",
    "shape": [1],
    "is_sparse": "false" == "true"
}

iris_label_meta = {
    "feature_name": "class",
    "dtype": "int64",
    "delimiter": "",
    "shape": [],
    "is_sparse": "false" == "true"
}


@unittest.skipUnless(testing.get_driver() == "maxcompute"
                     and testing.get_submitter() == "pai",
                     "skip non PAI tests")
class SubmitPAITrainTask(TestCase):
    def test_submit_pai_train_task(self):
        model_params = dict()
        model_params["hidden_units"] = [10, 20]
        model_params["n_classes"] = 3

        # feature_columns_code will be used to save the training information
        # together with the saved model.
        feature_columns_code = """{"feature_columns": [
            tf.feature_column.numeric_column("sepal_length", shape=[1]),
            tf.feature_column.numeric_column("sepal_width", shape=[1]),
            tf.feature_column.numeric_column("petal_length", shape=[1]),
            tf.feature_column.numeric_column("petal_width", shape=[1]),
        ]}"""
        feature_columns = eval(feature_columns_code)

        train(testing.get_datasource(),
              "DNNClassifier",
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
              "",
              model_params,
              "e2etest_pai_dnn",
              None,
              feature_columns=feature_columns,
              feature_column_names=iris_feature_column_names,
              feature_column_names_map=iris_feature_column_names_map,
              feature_metas=iris_feature_metas,
              label_meta=iris_label_meta,
              validation_metrics="Accuracy".split(","),
              batch_size=1,
              epoch=1,
              validation_steps=1,
              verbose=0,
              max_steps=None,
              validation_start_delay_secs=0,
              validation_throttle_secs=0,
              save_checkpoints_steps=100,
              log_every_n_iter=10,
              load_pretrained_model=False,
              is_pai=True,
              feature_columns_code=feature_columns_code,
              model_repo_image="",
              original_sql='''
SELECT * FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO e2etest_pai_dnn;''')

    def test_submit_pai_predict_task(self):
        predict(testing.get_datasource(),
                """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test""",
                "alifin_jtest_dev.pai_dnn_predict", "class", "e2etest_pai_dnn",
                {})

    def test_submit_pai_explain_task(self):
        explain(testing.get_datasource(),
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "alifin_jtest_dev.pai_dnn_explain_result", "e2etest_pai_dnn",
                {"label_col": "class"})

    def test_submit_xgb_train_task(self):
        model_params = {
            "booster": "gbtree",
            "eta": 0.4,
            "num_class": 3,
            "objective": "multi:softprob"
        }
        train_params = {"num_boost_round": 10}
        feature_columns_code = """
            xgboost_extended.feature_column.numeric_column(
                "sepal_length", shape=[1]),
            xgboost_extended.feature_column.numeric_column(
                "sepal_width", shape=[1]),
            xgboost_extended.feature_column.numeric_column(
                "petal_length", shape=[1]),
            xgboost_extended.feature_column.numeric_column(
                "petal_width", shape=[1])
        """
        train(testing.get_datasource(),
              "XGBoost",
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
              "select * from alifin_jtest_dev.sqlflow_iris_train",
              model_params,
              "e2etest_xgb_classify_model",
              None,
              train_params=train_params,
              feature_columns=eval("[%s]" % feature_columns_code),
              feature_metas=iris_feature_metas,
              label_meta=iris_label_meta,
              feature_column_names=iris_feature_column_names,
              feature_columns_code=feature_columns_code)

    def test_submit_pai_xgb_predict_task(self):
        predict(testing.get_datasource(),
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "alifin_jtest_dev.pai_xgb_predict", "class",
                "e2etest_xgb_classify_model", {})

    def test_submit_pai_xgb_explain_task(self):
        explain(testing.get_datasource(),
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                "alifin_jtest_dev.e2etest_xgb_explain_result",
                "e2etest_xgb_classify_model", {"label_col": "class"})

    def test_submit_pai_kmeans_train_task(self):
        train(
            testing.get_datasource(),
            "KMeans",
            "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
            "", {
                "excluded_columns": "class",
                "idx_table_name": "alifin_jtest_dev.e2e_test_kmeans_output_idx"
            },
            "e2e_test_kmeans",
            "",
            feature_column_names=[*iris_feature_column_names, "class"])

    def test_submit_pai_random_forest_train_task(self):
        train(testing.get_datasource(),
              "RandomForests",
              "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
              "", {
                  "tree_num": 3,
              },
              "e2e_test_random_forest",
              "",
              feature_column_names=iris_feature_column_names,
              label_meta=iris_label_meta)

    def test_submit_pai_random_forest_predict_task(self):
        predict(testing.get_datasource(),
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_test",
                "alifin_jtest_dev.pai_rf_predict", "class",
                "e2e_test_random_forest", {})

    def test_submit_pai_random_forest_explain_task(self):
        explain(testing.get_datasource(),
                "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                "alifin_jtest_dev.e2etest_random_forest_explain_result",
                "e2e_test_random_forest", {"label_col": "class"})

    def test_submit_pai_tf_evaluate_task(self):
        evaluate(testing.get_datasource(), "e2etest_pai_dnn",
                 "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                 "alifin_jtest_dev.e2etest_pai_dnn_evaluate_result",
                 {"validation.metrics": "Accuracy,Recall"})

    def test_submit_pai_xgb_evaluate_task(self):
        evaluate(testing.get_datasource(), "e2etest_xgb_classify_model",
                 "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
                 "alifin_jtest_dev.e2etest_pai_xgb_evaluate_result",
                 {"validation.metrics": "accuracy_score"})


if __name__ == "__main__":
    unittest.main()
