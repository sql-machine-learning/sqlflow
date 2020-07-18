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
from runtime.pai import submitter
from runtime.pai.cluster_conf import get_cluster_config


class SubmitterTestCase(TestCase):
    def test_get_oss_model_url(self):
        url = submitter.get_oss_model_url("user_a/model")
        self.assertEqual("oss://sqlflow-models/user_a/model", url)

    def test_get_datasource_dsn(self):
        ds = "odps://access_id:access_key@service.com/api?curr_project=test_ci&scheme=http"
        expected_dsn = "access_id:access_key@service.com/api?curr_project=test_ci&scheme=http"
        dsn = submitter.get_datasource_dsn(ds)
        self.assertEqual(expected_dsn, dsn)
        project = "test_ci"
        self.assertEqual(project, submitter.get_project(ds))

    def test_get_pai_tf_cmd(self):
        conf = get_cluster_config({})
        os.environ[
            "SQLFLOW_OSS_CHECKPOINT_CONFIG"] = '''{"arn":"arn", "host":"host"}'''
        cmd = submitter.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz -DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table -DhyperParameters='params.txt' "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?role_arn=arn/pai2osstestproject&host=host' "
            "-DgpuRequired='0'")
        self.assertEqual(expected, cmd)

        conf = get_cluster_config({"train.num_workers": 5})
        cmd = submitter.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz -DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table -DhyperParameters='params.txt' "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?role_arn=arn/pai2osstestproject&host=host' "
            r'''-Dcluster="{\"ps\": {\"count\": 1, \"cpu\": 200, \"gpu\": 0}, \"worker\": {\"count\": 5, \"cpu\": 400, \"gpu\": 0}}"'''
        )
        self.assertEqual(expected, cmd)
        del os.environ["SQLFLOW_OSS_CHECKPOINT_CONFIG"]


class SubmitPAITrainTask(TestCase):
    @unittest.skipUnless(testing.get_driver() == "maxcompute"
                         and testing.get_submitter() == "pai",
                         "skip non PAI tests")
    def test_submit_pai_train_task(self):

        feature_column_names = [
            "sepal_length",
            "sepal_width",
            "petal_length",
            "petal_width",
        ]

        # feature_column_names_map is used to determine the order of feature columns of each target:
        # e.g. when using DNNLinearCombinedClassifer.
        # feature_column_names_map will be saved to a single file when using PAI.
        feature_column_names_map = dict()
        feature_column_names_map["feature_columns"] = [
            "sepal_length",
            "sepal_width",
            "petal_length",
            "petal_width",
        ]

        feature_metas = dict()
        feature_metas["sepal_length"] = {
            "feature_name": "sepal_length",
            "dtype": "float32",
            "delimiter": "",
            "format": "",
            "shape": [1],
            "is_sparse": "false" == "true"
        }
        feature_metas["sepal_width"] = {
            "feature_name": "sepal_width",
            "dtype": "float32",
            "delimiter": "",
            "format": "",
            "shape": [1],
            "is_sparse": "false" == "true"
        }
        feature_metas["petal_length"] = {
            "feature_name": "petal_length",
            "dtype": "float32",
            "delimiter": "",
            "format": "",
            "shape": [1],
            "is_sparse": "false" == "true"
        }
        feature_metas["petal_width"] = {
            "feature_name": "petal_width",
            "dtype": "float32",
            "delimiter": "",
            "format": "",
            "shape": [1],
            "is_sparse": "false" == "true"
        }

        label_meta = {
            "feature_name": "class",
            "dtype": "int64",
            "delimiter": "",
            "shape": [],
            "is_sparse": "false" == "true"
        }

        model_params = dict()
        model_params["hidden_units"] = [10, 20]
        model_params["n_classes"] = 3

        # feature_columns_code will be used to save the training informations together
        # with the saved model.
        feature_columns_code = """{"feature_columns": [tf.feature_column.numeric_column("sepal_length", shape=[1]),
        tf.feature_column.numeric_column("sepal_width", shape=[1]),
        tf.feature_column.numeric_column("petal_length", shape=[1]),
        tf.feature_column.numeric_column("petal_width", shape=[1])]}"""
        feature_columns = eval(feature_columns_code)

        submitter.submit_pai_tf_train(
            testing.get_datasource(),
            "DNNClassifier",
            "SELECT * FROM alifin_jtest_dev.sqlflow_iris_train",
            "",
            model_params,
            "e2etest_pai_dnn",
            None,
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_column_names_map=feature_column_names_map,
            feature_metas=feature_metas,
            label_meta=label_meta,
            validation_metrics="Accuracy".split(","),
            save="model_save",
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
            original_sql=
            '''SELECT * FROM alifin_jtest_dev.sqlflow_test_iris_train
    TO TRAIN DNNClassifier
    WITH model.n_classes = 3, model.hidden_units = [10, 20]
    LABEL class
    INTO e2etest_pai_dnn;''')

    @unittest.skipUnless(testing.get_driver() == "maxcompute"
                         and testing.get_submitter() == "pai",
                         "skip non PAI tests")
    def test_submit_pai_predict_task(self):
        submitter.submit_pai_tf_predict(
            testing.get_datasource(),
            """SELECT * FROM alifin_jtest_dev.sqlflow_iris_test""",
            "alifin_jtest_dev.pai_dnn_predict", "class", "e2etest_pai_dnn", {})


if __name__ == "__main__":
    unittest.main()
