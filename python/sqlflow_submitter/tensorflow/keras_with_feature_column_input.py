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

import tensorflow as tf


class WrappedKerasModel(tf.keras.Model):
    def __init__(self, keras_model, model_params, feature_columns):
        super(WrappedKerasModel, self).__init__()
        self.sub_model = keras_model(**model_params)
        self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)

    def __call__(self, inputs, training=True):
        x = self.feature_layer(inputs)
        return self.sub_model.__call__(x, training=training)
