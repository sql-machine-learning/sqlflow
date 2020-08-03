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

from estimator_example import (datasource, feature_column_names,
                               feature_columns, feature_metas, label_meta)
from runtime.tensorflow.explain import explain
from runtime.tensorflow.train import train

if __name__ == "__main__":
    # Train and explain BoostedTreesClassifier
    train(datasource=datasource,
          estimator_string="BoostedTreesClassifier",
          select="SELECT * FROM iris.train where class!=2",
          validation_select="SELECT * FROM iris.test where class!=2",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_batches_per_layer": 1,
              "n_classes": 2,
              "n_trees": 50,
              "center_bias": True
          },
          save="btmodel",
          batch_size=100,
          epoch=20,
          verbose=0)

    explain(datasource=datasource,
            estimator_string="BoostedTreesClassifier",
            select="SELECT * FROM iris.test where class!=2",
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={
                "n_batches_per_layer": 1,
                "n_classes": 2,
                "n_trees": 50,
                "center_bias": True
            },
            save="btmodel",
            plot_type='bar',
            result_table="")
    shutil.rmtree("btmodel")

    # Train and explain DNNClassifier
    train(datasource=datasource,
          estimator_string="DNNClassifier",
          select="SELECT * FROM iris.train",
          validation_select="SELECT * FROM iris.test",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "n_classes": 3,
              "hidden_units": [100, 100],
          },
          save="dnnmodel",
          batch_size=100,
          epoch=20,
          verbose=0)

    explain(datasource=datasource,
            estimator_string="DNNClassifier",
            select="SELECT * FROM iris.test LIMIT 10",
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={
                "n_classes": 3,
                "hidden_units": [100, 100],
            },
            save="dnnmodel",
            plot_type='bar',
            result_table="")
    shutil.rmtree("dnnmodel")

    # Train and explain DNNRegressor
    train(datasource=datasource,
          estimator_string="DNNRegressor",
          select="SELECT * FROM iris.train",
          validation_select="SELECT * FROM iris.test",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={
              "hidden_units": [100, 100],
          },
          save="dnnmodel",
          batch_size=100,
          epoch=20,
          verbose=0)

    explain(datasource=datasource,
            estimator_string="DNNRegressor",
            select="SELECT * FROM iris.test LIMIT 10",
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={
                "hidden_units": [100, 100],
            },
            save="dnnmodel",
            plot_type='bar',
            result_table="")
    shutil.rmtree("dnnmodel")

    # Train and explain LinearRegressor
    train(datasource=datasource,
          estimator_string="LinearRegressor",
          select="SELECT * FROM iris.train",
          validation_select="SELECT * FROM iris.test",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params={},
          save="lrmodel",
          batch_size=100,
          epoch=20,
          verbose=0)

    explain(datasource=datasource,
            estimator_string="LinearRegressor",
            select="SELECT * FROM iris.test LIMIT 10",
            feature_columns=feature_columns,
            feature_column_names=feature_column_names,
            feature_metas=feature_metas,
            label_meta=label_meta,
            model_params={},
            save="lrmodel",
            plot_type='bar',
            result_table="")
    shutil.rmtree("lrmodel")
