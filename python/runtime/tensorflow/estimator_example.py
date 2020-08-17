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

# NOTE: this file is used by train_predict_test.py, do **NOT** delete!
import shutil

import runtime.testing as testing
import tensorflow as tf
from runtime.tensorflow.predict import pred
from runtime.tensorflow.train import train

datasource = testing.get_datasource()
select = "SELECT * FROM iris.train;"
validate_select = "SELECT * FROM iris.test;"
select_binary = "SELECT * FROM iris.train WHERE class!=2;"
validate_select_binary = "SELECT * FROM iris.test WHERE class!=2;"
feature_column_names = [
    "sepal_length", "sepal_width", "petal_length", "petal_width"
]
feature_column_names_map = {
    "feature_columns":
    ["sepal_length", "sepal_width", "petal_length", "petal_width"]
}

feature_columns = {
    "feature_columns": [
        tf.feature_column.numeric_column("sepal_length", shape=[1]),
        tf.feature_column.numeric_column("sepal_width", shape=[1]),
        tf.feature_column.numeric_column("petal_length", shape=[1]),
        tf.feature_column.numeric_column("petal_width", shape=[1])
    ]
}

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

if __name__ == "__main__":
    # tf.python.training.basic_session_run_hooks.LoggingTensorHook
    # = runtime.tensorflow.train.PrintTensorsHook
    train(datasource=datasource,
          estimator_string="DNNClassifier",
          select=select,
          validation_select=validate_select,
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 3,
              "hidden_units": [10, 20]
          },
          save="mymodel",
          batch_size=1,
          epoch=3,
          verbose=0)
    train(datasource=datasource,
          estimator_string="DNNClassifier",
          select=select_binary,
          validation_select=validate_select_binary,
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 2,
              "hidden_units": [10, 20]
          },
          save="mymodel_binary",
          batch_size=1,
          epoch=3,
          verbose=1)
    pred(datasource=datasource,
         estimator_string="DNNClassifier",
         select=select,
         result_table="iris.predict",
         feature_columns=feature_columns,
         feature_column_names=feature_column_names,
         feature_column_names_map=feature_column_names_map,
         train_label_name=label_meta["feature_name"],
         result_col_name=label_meta["feature_name"],
         feature_metas=feature_metas,
         model_params={
             "n_classes": 3,
             "hidden_units": [10, 20]
         },
         save="mymodel",
         batch_size=1)
    shutil.rmtree("mymodel")
    shutil.rmtree("mymodel_binary")
