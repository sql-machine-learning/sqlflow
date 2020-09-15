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

import shutil

import tensorflow as tf
from runtime.tensorflow.estimator_example import datasource
from runtime.tensorflow.predict import pred
from runtime.tensorflow.train import train

# NOTE: this file is used by train_predict_test.py, do **NOT** delete!

select = "select * from housing.train"
validation_select = "select * from housing.test"

feature_column_names = ["f%d" % i for i in range(1, 14)]
feature_column_names_map = {
    "feature_columns": ["f%d" % i for i in range(1, 14)]
}

feature_columns = {
    "feature_columns": [
        tf.feature_column.numeric_column("f%d" % i, shape=[1])
        for i in range(1, 14)
    ]
}

feature_metas = {
    "f1": {
        "feature_name": "f1",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f2": {
        "feature_name": "f2",
        "dtype": "int64",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f3": {
        "feature_name": "f3",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f4": {
        "feature_name": "f4",
        "dtype": "int64",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f5": {
        "feature_name": "f5",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f6": {
        "feature_name": "f6",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f7": {
        "feature_name": "f7",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f8": {
        "feature_name": "f8",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f9": {
        "feature_name": "f9",
        "dtype": "int64",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f10": {
        "feature_name": "f10",
        "dtype": "int64",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f11": {
        "feature_name": "f11",
        "dtype": "int64",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f12": {
        "feature_name": "f12",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    },
    "f13": {
        "feature_name": "f13",
        "dtype": "float32",
        "delimiter": "",
        "shape": [1],
        "is_sparse": "false" == "true"
    }
}

label_meta = {
    "feature_name": "target",
    "dtype": "float32",
    "delimiter": "",
    "shape": [],
    "is_sparse": "false" == "true"
}

if __name__ == "__main__":
    train(datasource=datasource,
          estimator_string="sqlflow_models.DNNRegressor",
          select=select,
          validation_select=validation_select,
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={"hidden_units": [10, 20]},
          validation_metrics=["CategoricalAccuracy"],
          save="myregmodel_keras",
          batch_size=1,
          epoch=3,
          verbose=0)
    pred(datasource=datasource,
         estimator_string="sqlflow_models.DNNRegressor",
         select=validation_select,
         result_table="housing.predict",
         feature_columns=feature_columns,
         feature_column_names=feature_column_names,
         feature_column_names_map=feature_column_names_map,
         train_label_name=label_meta["feature_name"],
         result_col_name=label_meta["feature_name"],
         feature_metas=feature_metas,
         model_params={"hidden_units": [10, 20]},
         save="myregmodel_keras",
         batch_size=1)
    shutil.rmtree("myregmodel_keras")
