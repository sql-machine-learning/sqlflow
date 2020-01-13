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

# NOTE: this file is used by train_predict_test.py, do **NOT** delete!

import sqlflow_models
from sqlflow_submitter.tensorflow.estimator_example import (
    datasource, feature_column_names, feature_columns, feature_metas,
    label_meta, select, validate_select)
from sqlflow_submitter.tensorflow.predict import pred
from sqlflow_submitter.tensorflow.train import train

if __name__ == "__main__":
    train(datasource=datasource,
          estimator=sqlflow_models.DNNClassifier,
          select=select,
          validate_select=validate_select,
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 3,
              "hidden_units": [10, 20]
          },
          save="mymodel_keras",
          batch_size=1,
          epochs=3,
          verbose=0)
    pred(datasource=datasource,
         estimator=sqlflow_models.DNNClassifier,
         select=select,
         result_table="iris.predict",
         feature_columns=feature_columns,
         feature_column_names=feature_column_names,
         feature_metas=feature_metas,
         label_meta=label_meta,
         model_params={
             "n_classes": 3,
             "hidden_units": [10, 20]
         },
         save="mymodel_keras",
         batch_size=1)
