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

import runtime.model.oss as oss
import runtime.temp_file as temp_file
from runtime.model import EstimatorType, Model
from runtime.testing import get_datasource


class TestModel(unittest.TestCase):
    def test_save_load_db(self):
        table = "sqlflow_models.test_model"
        meta = {"model_params": {"n_classes": 3}}
        m = Model(EstimatorType.XGBOOST, meta)
        datasource = get_datasource()

        # save mode
        with temp_file.TemporaryDirectory() as d:
            m.save_to_db(datasource, table, d)

        # load model
        with temp_file.TemporaryDirectory() as d:
            m = Model.load_from_db(datasource, table, d)
            self.assertEqual(m._meta, meta)

    @unittest.skipUnless(
        os.getenv("SQLFLOW_OSS_AK") and os.getenv("SQLFLOW_OSS_SK"),
        "skip when SQLFLOW_OSS_AK or SQLFLOW_OSS_SK is not set")
    def test_save_load_oss(self):
        bucket = oss.get_models_bucket()
        meta = {"model_params": {"n_classes": 3}}
        m = Model(EstimatorType.XGBOOST, meta)

        oss_dir = "unknown/model_test_dnn_classifier/"
        oss_model_path = "oss://%s/%s" % (bucket.bucket_name, oss_dir)

        oss.delete_oss_dir_recursive(bucket, oss_dir)

        # save model
        def save_to_oss():
            with temp_file.TemporaryDirectory() as d:
                m.save_to_oss(oss_model_path, d)

        # load model
        def load_from_oss():
            with temp_file.TemporaryDirectory() as d:
                return Model.load_from_oss(oss_model_path, d)

        with self.assertRaises(Exception):
            load_from_oss()

        save_to_oss()
        m = load_from_oss()
        self.assertEqual(m._meta, meta)


if __name__ == '__main__':
    unittest.main()
