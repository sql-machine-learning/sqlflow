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
            "SQLFLOW_OSS_CHECKPOINT_DIR"] = '''{"Arn":"arn", "Host":"host"}'''
        cmd = submitter.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project",
            "/tmp")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz -DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table -DhyperParameters=\"params.txt\" "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?role_arn=arn/pai2oss_test_project&host=host' "
            "-DgpuRequired='0'")
        self.assertEqual(expected, cmd)

        conf = get_cluster_config({"train.num_workers": 5})
        cmd = submitter.get_pai_tf_cmd(
            conf, "job.tar.gz", "params.txt", "entry.py", "my_dnn_model",
            "user1/my_dnn_model", "test_project.input_table",
            "test_project.val_table", "test_project.res_table", "test_project",
            "/tmp")
        expected = (
            "pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 "
            "-DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=job.tar.gz -DentryFile=entry.py "
            "-Dtables=odps://test_project/tables/input_table,odps://test_project/tables/val_table "
            "-Doutputs=odps://test_project/tables/res_table -DhyperParameters=\"params.txt\" "
            "-DcheckpointDir='oss://sqlflow-models/user1/my_dnn_model/?role_arn=arn/pai2oss_test_project&host=host' "
            r'''-Dcluster="{\"ps\": {\"count\": 1, \"cpu\": 200, \"gpu\": 0}, \"worker\": {\"count\": 5, \"cpu\": 400, \"gpu\": 0}}"'''
        )
        self.assertEqual(expected, cmd)
        del os.environ["SQLFLOW_OSS_CHECKPOINT_DIR"]


if __name__ == "__main__":
    unittest.main()
