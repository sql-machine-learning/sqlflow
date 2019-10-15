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

from sqlflow_submitter.tensorflow.train import train
from sqlflow_submitter.tensorflow.predict import pred

datasource = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
select = "SELECT * FROM iris.train;"
validate_select = "SELECT * FROM iris.test;"
feature_column_names = [
"sepal_length",
"sepal_width",
"petal_length",
"petal_width"]

feature_column_code = '''feature_columns=[tf.feature_column.numeric_column("sepal_length", shape=[1]),
tf.feature_column.numeric_column("sepal_width", shape=[1]),
tf.feature_column.numeric_column("petal_length", shape=[1]),
tf.feature_column.numeric_column("petal_width", shape=[1])]'''

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
    }}
label_meta = {
    "feature_name": "class",
    "dtype": "int64",
    "delimiter": "",
    "shape": [1],
    "is_sparse": "false" == "true"
}

if __name__ == "__main__":
    train(is_keara_model=False,
        datasource=datasource,
        estimator="tf.estimator.DNNClassifier",
        select=select,
        validate_select=validate_select,
        feature_column_code=feature_column_code,
        feature_column_names=feature_column_names,
        feature_metas=feature_metas,
        label_meta=label_meta,
        model_params={"n_classes": 3, "hidden_units":[10,20]},
        save="mymodel",
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
