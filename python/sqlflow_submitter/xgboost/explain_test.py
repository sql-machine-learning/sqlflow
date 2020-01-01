# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
import numpy as np
from sqlflow_submitter.xgboost.explain import explain as xgb_explain 
from sqlflow_submitter.xgboost.explain import xgb_shap_values, xgb_shap_dataset
from sqlflow_submitter.xgboost.train import train as xgb_train

datasource = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
select = "SELECT * FROM boston.train;"

feature_field_meta=[
  {'name': 'crim', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'zn', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'indus', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'chas', 'dtype': 0, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'nox', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'rm', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'age', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'dis', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'rad', 'dtype': 0, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'tax', 'dtype': 0, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'ptratio', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'b', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0},
  {'name': 'lstat', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0}
]

label_field_meta= {'name': 'medv', 'dtype': 1, 'delimiter': '', 'shape': [1], 'is_sparse': False, 'vocabulary': None, 'MaxID': 0}

class ExplainXGBModeTestCase(TestCase):
    def tearDown(self):
        os.remove('my_model')

    def test_explain(self):
        xgb_train(
          datasource=datasource,
          select=select,
          model_params={"objective":"reg:squarederror"},
          train_params={"num_boost_round": 30},
          feature_field_meta=feature_field_meta,
          label_field_meta=label_field_meta,
          validation_select=""
        )
        feature_column_names = [k["name"] for k in feature_field_meta]
        feature_specs = {k['name']: k for k in feature_field_meta}
        x = xgb_shap_dataset(datasource, select, feature_column_names, label_field_meta['name'], feature_specs)

        shap_values = xgb_shap_values(x)
        expected_features=['chas', 'zn', 'rad', 'indus', 'b', 'tax', 'ptratio', 'age', 'nox', 'crim', 'dis', 'rm', 'lstat']
        actural_features=[x.columns[idx] for idx in np.argsort(np.abs(shap_values).mean(0))]

        self.assertEqual(expected_features, actural_features)


if __name__ == '__main__':
    unittest.main()
