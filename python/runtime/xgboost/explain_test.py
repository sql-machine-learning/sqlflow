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

import numpy as np
import runtime.testing as testing
from runtime.xgboost.explain import xgb_shap_dataset, xgb_shap_values
from runtime.xgboost.train import train as xgb_train

datasource = testing.get_datasource()

select = "SELECT * FROM boston.train;"

feature_field_meta = [{
    'feature_name': 'crim',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'zn',
    'dtype': "int32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'indus',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'chas',
    'dtype': "int32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'nox',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'rm',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'age',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'dis',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'rad',
    'dtype': "int32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'tax',
    'dtype': "int32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'ptratio',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'b',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}, {
    'feature_name': 'lstat',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}]

label_field_meta = {
    'feature_name': 'medv',
    'dtype': "float32",
    'delimiter': '',
    'shape': [1],
    'is_sparse': False,
    'vocabulary': None,
    'MaxID': 0
}


class ExplainXGBModeTestCase(TestCase):
    def tearDown(self):
        os.remove('my_model')

    def test_explain(self):
        feature_column_names = [k["feature_name"] for k in feature_field_meta]
        feature_metas = {k['feature_name']: k for k in feature_field_meta}
        xgb_train(datasource=datasource,
                  select=select,
                  model_params={"objective": "reg:squarederror"},
                  train_params={"num_boost_round": 30},
                  feature_metas=feature_metas,
                  feature_column_names=feature_column_names,
                  label_meta=label_field_meta,
                  validation_select="")
        # TODO(Yancey1989): keep shap codegen consistent with XGBoost
        label_field_meta['name'] = label_field_meta['feature_name']
        x = xgb_shap_dataset(datasource, select, feature_column_names,
                             label_field_meta, feature_metas, False, "")

        shap_values = xgb_shap_values(x)[0]
        expected_features = [
            'chas', 'zn', 'rad', 'indus', 'b', 'tax', 'ptratio', 'age', 'nox',
            'crim', 'dis', 'rm', 'lstat'
        ]

        actual_features = [
            x.columns[idx] for idx in np.argsort(np.abs(shap_values).mean(0))
        ]

        self.assertEqual(expected_features, actual_features)


if __name__ == '__main__':
    unittest.main()
