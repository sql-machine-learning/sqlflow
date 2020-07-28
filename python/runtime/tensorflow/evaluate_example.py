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

from estimator_example import (datasource, feature_column_names,
                               feature_columns, feature_metas, label_meta)
from runtime.tensorflow.evaluate import evaluate
from runtime.tensorflow.train import train

if __name__ == "__main__":
    # Test evaluation on an estimator model
    train(datasource=datasource,
          estimator_string="tf.estimator.DNNClassifier",
          select="SELECT * FROM iris.train where class!=2",
          validation_select="SELECT * FROM iris.test where class!=2",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 2,
              "hidden_units": [128, 32]
          },
          save="bin_model",
          batch_size=10,
          epoch=20,
          verbose=0)
    # FIXME(typhoonzero): need to re-create result table: iris.evaluate_result?
    evaluate(
        datasource=datasource,
        estimator_string="tf.estimator.DNNClassifier",
        select="SELECT * FROM iris.test where class!=2",
        result_table="",
        feature_columns=feature_columns,
        feature_column_names=feature_column_names,
        feature_metas=feature_metas,
        label_meta=label_meta,
        model_params={
            "n_classes": 2,
            "hidden_units": [128, 32]
        },
        validation_metrics=["Accuracy", "AUC"],
        save="bin_model",
        batch_size=1,
        validation_steps=None,
        verbose=0,
    )

    # Test evaluation on a keras model
    train(datasource=datasource,
          estimator_string="sqlflow_models.DNNClassifier",
          select="SELECT * FROM iris.train where class!=2",
          validation_select="SELECT * FROM iris.test where class!=2",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 2,
              "hidden_units": [128, 32]
          },
          save="bin_model_keras",
          batch_size=10,
          epoch=20,
          verbose=0)
    # FIXME(typhoonzero): need to re-create result table: iris.evaluate_result?
    evaluate(
        datasource=datasource,
        estimator_string="sqlflow_models.DNNClassifier",
        select="SELECT * FROM iris.test where class!=2",
        result_table="",
        feature_columns=feature_columns,
        feature_column_names=feature_column_names,
        feature_metas=feature_metas,
        label_meta=label_meta,
        model_params={
            "n_classes": 2,
            "hidden_units": [128, 32]
        },
        validation_metrics=["BinaryAccuracy", "AUC"],
        save="bin_model_keras",
        batch_size=1,
        validation_steps=None,
        verbose=0,
    )
