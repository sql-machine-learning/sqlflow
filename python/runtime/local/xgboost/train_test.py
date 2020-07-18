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
from unittest import TestCase

import runtime.testing as testing
from runtime.local.xgboost import train
from runtime.xgboost.dataset import xgb_dataset

# iris dataset features meta
# TODO(yancey1989): implement runtime.feature_derivation API to generate the following
# feature metas
feature_metas = {
    "sepal_length": {
        "feature_name": "sepal_length",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "sepal_width": {
        "feature_name": "sepal_width",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "petal_length": {
        "feature_name": "petal_length",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "petal_width": {
        "feature_name": "petal_width",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    }
}

label_meta = {
    "feature_name": "class",
    "dtype": "int64",
    "delimiter": "",
    "shape": [],
    "is_sparse": "false" == "true"
}


class TestXGBoostTrain(TestCase):
    @unittest.skipUnless(testing.get_driver() == "mysql",
                         "skip non mysql tests")
    def test_train(self):
        ds = testing.get_datasource()
        select = "SELECT * FROM iris.train"
        val_select = "SELECT * FROM iris.test"
        feature_column_names = [
            feature_metas[k]["feature_name"] for k in feature_metas
        ]
        is_pai = False
        pai_train_table = ""
        train_params = {"num_boost_round": 20}
        model_params = {"num_classes": 3}
        with tempfile.TemporaryDirectory() as tmp_dir_name:
            train_fn = os.path.join(tmp_dir_name, 'train.txt')
            val_fn = os.path.join(tmp_dir_name, 'val.txt')
            dtrain = xgb_dataset(ds, train_fn, select, feature_metas,
                                 feature_column_names, label_meta, is_pai,
                                 pai_train_table)
            dval = xgb_dataset(ds, val_fn, val_select, feature_metas,
                               feature_column_names, label_meta, is_pai,
                               pai_train_table)
            eval_result = train(dtrain, train_params, model_params, dval)
            self.assertLess(eval_result['train']['rmse'][-1], 0.01)
            self.assertLess(eval_result['validate']['rmse'][-1], 0.01)


if __name__ == '__main__':
    unittest.main()
