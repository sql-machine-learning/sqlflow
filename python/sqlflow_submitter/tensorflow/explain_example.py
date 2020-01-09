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

import sqlflow_submitter
from sqlflow_submitter.tensorflow.train import train
from sqlflow_submitter.tensorflow.predict import pred
from sqlflow_submitter.tensorflow.explain import explain
from estimator_example import datasource, select_binary, validate_select_binary, feature_column_names, feature_columns, feature_metas, label_meta
import tensorflow as tf

if __name__ == "__main__":
    train(datasource=datasource,
          estimator=tf.estimator.BoostedTreesClassifier,
          select="SELECT * FROM iris.train where class!=2",
          validate_select="SELECT * FROM iris.test where class!=2",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={"n_batches_per_layer": 8, "n_classes": 2, "n_trees": 50, "center_bias": True},
          save="btmodel",
          batch_size=8,
          epochs=20,
          verbose=0)
    explain(datasource=datasource,
            estimator_cls=tf.estimator.BoostedTreesClassifier,
            select="SELECT * FROM iris.test where class!=2",
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={"n_batches_per_layer": 8, "n_classes": 2, "n_trees": 50, "center_bias": True},
            save="btmodel",
            is_pai=False,
            plot_type='bar',
            result_table="iris.explain_result")
