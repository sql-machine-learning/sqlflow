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

import numpy as np
import pandas as pd
import xgboost
from jpmml_evaluator import make_evaluator
from jpmml_evaluator.pyjnius import PyJNIusBackend, jnius_configure_classpath
from runtime.local.xgboost_submitter.save import save_model_to_local_file

# Configure JVM
jnius_configure_classpath()

# Construct a PyJNIus backend
PMML_BACKEND = PyJNIusBackend()


# TODO(sneaxiy): Add XGBRanker unittest. XGBRanker requires group information
# but we do not support group now.
class TestXGBoostModelSavingBase(unittest.TestCase):
    def tearDown(self):
        filename = self.filename()
        if filename is not None and os.path.exists(filename):
            os.remove(filename)
            os.remove(self.pmml_filename())

    def filename(self):
        pass

    def pmml_filename(self):
        return "{}.pmml".format(self.filename())

    def batch_size(self):
        return 1024

    def feature_size(self):
        return 32

    def save_and_load_model(self, booster, params):
        save_model_to_local_file(booster, params, self.filename())
        self.assertTrue(os.path.exists(self.filename()))
        self.assertTrue(os.path.exists(self.pmml_filename()))

        loaded_booster = xgboost.Booster({"n_thread": 4})
        loaded_booster.load_model(self.filename())

        pmml_evaluator = make_evaluator(PMML_BACKEND,
                                        self.pmml_filename()).verify()
        return loaded_booster, pmml_evaluator

    def validate_predict_result(self, booster, params, x):
        loaded_booster, pmml_evaluator = self.save_and_load_model(
            booster, params)

        original_model_y_predict = booster.predict(xgboost.DMatrix(x))
        loaded_model_y_predict = booster.predict(xgboost.DMatrix(x))
        self.assertTrue(
            np.array_equal(original_model_y_predict, loaded_model_y_predict))

        column_names = [
            field.name for field in pmml_evaluator.getInputFields()
        ]

        # column_names is like: "x1", "x2", "x3", ....
        column_index = [int(name[1:]) - 1 for name in column_names]

        # The unused column in boosting tree would be discarded when saving
        # PMML. Supposing N is the number of used columns in boosting tree.
        # PMML evaluator only accepts the first N columns of
        # pandas.DataFrame as its input, and would not check whether
        # the column name of pandas.DataFrame matches the saved column
        # name in PMML file. Therefore, we would pick the used columns
        # of `x` instead of all columns as the input of the PMML evaluator.
        pmml_input = pd.DataFrame(data=x[:, column_index],
                                  columns=column_names)

        pmml_predict = np.array(pmml_evaluator.evaluateAll(pmml_input)["y"])

        objective = params.get("objective")
        if objective.startswith("binary:"):
            booster_label = (original_model_y_predict >= 0.5).astype("int64")
            pmml_label = pmml_predict.astype("int64")
        elif objective.startswith("multi:"):
            booster_label = np.array(original_model_y_predict).astype("int64")
            pmml_label = pmml_predict.astype("int64")
        else:
            booster_label = original_model_y_predict
            pmml_label = pmml_predict

        self.assertTrue(np.array_equal(booster_label, pmml_label))


class TestXGBoostBinaryClassifierModelSaving(TestXGBoostModelSavingBase):
    def filename(self):
        return "xgboost_binary_classifier_model"

    def test_main(self):
        batch_size = self.batch_size()
        feature_size = self.feature_size()

        params = {"objective": "binary:logistic"}

        x = np.random.random(size=[batch_size, feature_size]).astype("float32")
        y = np.random.randint(low=0, high=2, size=[batch_size]).astype("int64")
        dtrain = xgboost.DMatrix(x, y)

        booster = xgboost.train(params, dtrain)
        self.validate_predict_result(booster, params, x)


class TestXGBoostMultiClassifierModelSaving(TestXGBoostModelSavingBase):
    def filename(self):
        return "xgboost_multi_classifier_model"

    def test_main(self):
        num_class = 4
        batch_size = self.batch_size()
        feature_size = self.feature_size()

        params = {"objective": "multi:softmax", "num_class": num_class}

        x = np.random.random(size=[batch_size, feature_size]).astype("float32")
        y = np.random.randint(low=0, high=num_class,
                              size=[batch_size]).astype("int64")
        dtrain = xgboost.DMatrix(x, y)

        booster = xgboost.train(params, dtrain)
        self.validate_predict_result(booster, params, x)


class TestXGBoostRegressorModelSaving(TestXGBoostModelSavingBase):
    def filename(self):
        return "xgboost_regressor_model"

    def test_main(self):
        batch_size = self.batch_size()
        feature_size = self.feature_size()

        params = {"objective": "reg:squarederror"}

        x = np.random.random(size=[batch_size, feature_size]).astype("float32")
        y = np.random.random(size=[batch_size]).astype("float32")
        dtrain = xgboost.DMatrix(x, y)

        booster = xgboost.train(params, dtrain)
        self.validate_predict_result(booster, params, x)


if __name__ == "__main__":
    unittest.main()
