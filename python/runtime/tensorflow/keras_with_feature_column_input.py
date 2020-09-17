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

import inspect

import tensorflow as tf
from runtime.diagnostics import init_model


class WrappedKerasModel(tf.keras.Model):
    def __init__(self, keras_model, model_params, feature_columns):
        super(WrappedKerasModel, self).__init__()
        self.sub_model = keras_model(**model_params)
        self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)

    def call(self, inputs, training=True):
        x = self.feature_layer(inputs)
        return self.sub_model(x, training=training)


def init_model_with_feature_column(estimator,
                                   model_params,
                                   has_none_optimizer=False):
    """Check if estimator have argument "feature_column" and initialize the model
       by wrapping the keras model if no "feature_column" argument detected.

       NOTE: initalize estimator model can also use this function since
       estimators all have "feature_column" argument.
    """
    if inspect.isclass(estimator):
        argspec = inspect.getargspec(estimator.__init__)
    else:
        argspec = inspect.getargspec(estimator)

    if "feature_columns" not in argspec.args and not has_none_optimizer:
        feature_columns = model_params["feature_columns"]
        del model_params["feature_columns"]
        classifier = WrappedKerasModel(estimator, model_params,
                                       feature_columns)
    else:
        classifier = init_model(estimator, model_params)
    return classifier
