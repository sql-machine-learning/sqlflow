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

from runtime.model.model import Model
from runtime.step.tensorflow.train import train_step


def train(original_sql,
          model_image,
          estimator_string,
          datasource,
          select,
          validation_select,
          model_params,
          train_params,
          validation_params,
          feature_column_map,
          label_column,
          save,
          load=None):
    if load:
        Model.load_from_db(datasource, load)
        load = "model_save"
    else:
        load = None

    train_step(original_sql=original_sql,
               model_image=model_image,
               estimator_string=estimator_string,
               datasource=datasource,
               select=select,
               validation_select=validation_select,
               model_params=model_params,
               train_params=train_params,
               validation_params=validation_params,
               feature_column_map=feature_column_map,
               label_column=label_column,
               save=save,
               load=load)
