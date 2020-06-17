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
# NOTE: this file is used by train_predict_test.py, do **NOT** delete!
import shutil

import sqlflow_models
from sqlflow_submitter.tensorflow.estimator_example import (
    datasource, feature_column_names, feature_column_names_map,
    feature_columns, feature_metas, label_meta, select, validate_select)
from sqlflow_submitter.tensorflow.predict import pred
from sqlflow_submitter.tensorflow.train import train

native_keras_code = """import tensorflow as tf

class RawDNNClassifier(tf.keras.Model):
    def __init__(self, hidden_units=[100,100], n_classes=3):
        super(RawDNNClassifier, self).__init__()
        self.feature_layer = None
        self.n_classes = n_classes
        self.hidden_layers = []
        for hidden_unit in hidden_units:
            self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit, activation='relu'))
        if self.n_classes == 2:
            pred_act = 'sigmoid'
            self.loss = 'binary_crossentropy'
            n_out = 1
        else:
            pred_act = 'softmax'
            self.loss = 'categorical_crossentropy'
            n_out = self.n_classes
        self.prediction_layer = tf.keras.layers.Dense(n_out, activation=pred_act)

    def call(self, inputs, training=True):
        if self.feature_layer is not None:
            x = self.feature_layer(inputs)
        else:
            x = tf.keras.layers.Flatten()(inputs)
        for hidden_layer in self.hidden_layers:
            x = hidden_layer(x)
        return self.prediction_layer(x)
"""

if __name__ == "__main__":
    train(datasource=datasource,
          estimator_string="sqlflow_models.DNNClassifier",
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
          validation_metrics=["CategoricalAccuracy"],
          save="mymodel_keras",
          batch_size=1,
          epoch=3,
          verbose=0)
    pred(datasource=datasource,
         estimator_string="sqlflow_models.DNNClassifier",
         select=select,
         result_table="iris.predict",
         feature_columns=feature_columns,
         feature_column_names=feature_column_names,
         feature_column_names_map=feature_column_names_map,
         result_col_name=label_meta["feature_name"],
         feature_metas=feature_metas,
         model_params={
             "n_classes": 3,
             "hidden_units": [10, 20]
         },
         save="mymodel_keras",
         batch_size=1)
    os.unlink("mymodel_keras")

    train(datasource=datasource,
          estimator_string="sqlflow_models.RawDNNClassifier",
          select=select,
          validation_select=validate_select,
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 3,
              "hidden_units": [10, 20],
              "loss": "sparse_categorical_crossentropy"
          },
          validation_metrics=["CategoricalAccuracy"],
          save="mymodel_keras",
          batch_size=1,
          epoch=3,
          verbose=0)
    pred(datasource=datasource,
         estimator_string="sqlflow_models.RawDNNClassifier",
         select=select,
         result_table="iris.predict",
         feature_columns=feature_columns,
         feature_column_names=feature_column_names,
         feature_column_names_map=feature_column_names_map,
         result_col_name=label_meta["feature_name"],
         feature_metas=feature_metas,
         model_params={
             "n_classes": 3,
             "hidden_units": [10, 20]
         },
         save="mymodel_keras",
         batch_size=1)
    os.unlink("mymodel_keras")
