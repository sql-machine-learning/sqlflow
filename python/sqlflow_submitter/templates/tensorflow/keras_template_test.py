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

import unittest
from unittest import TestCase

from sqlflow_submitter.templates.tensorflow.template_test import datasource, select, validate_select, feature_column_names, feature_column_code, feature_metas, label_meta
from sqlflow_submitter.templates.tensorflow.train_template import train
from sqlflow_submitter.templates.tensorflow.pred_template import pred
import tensorflow as tf

class TestDB(TestCase):
    def test_pred(self):
        train(is_keara_model=True,
            datasource=datasource,
            estimator="sqlflow_models.DNNClassifier",
            select=select,
            validate_select=validate_select,
            feature_column_code=feature_column_code,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={"n_classes": 3, "hidden_units":[10,20]},
            save="mymodel_keras",
            batch_size=1,
            epochs=1,
            verbose=0)
        pred(is_keara_model=False,
            datasource=datasource,
            estimator="tf.estimator.DNNClassifier",
            select=select,
            result_table="iris.predict",
            feature_column_code=feature_column_code,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={"n_classes": 3, "hidden_units":[10,20]},
            save="mymodel",
            batch_size=1,
            epochs=1,
            verbose=0)

if __name__ == '__main__':
    unittest.main()
