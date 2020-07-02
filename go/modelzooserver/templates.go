// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modelzooserver

const sampleInitCode = `from .my_test_model import DNNClassifier`

const sampleDockerfile = `FROM ubuntu:bionic
ADD my_test_models /models/my_test_models/
ENV PYTHONPATH=/models:/usr/local/sqlflow/python
`

const sampleModelCode = `import tensorflow as tf

class DNNClassifier(tf.keras.Model):
    def __init__(self, feature_columns=None, hidden_units=[100,100], n_classes=3):
        """DNNClassifier
        :param feature_columns: feature columns.
        :type feature_columns: list[tf.feature_column].
        :param hidden_units: number of hidden units.
        :type hidden_units: list[int].
        :param n_classes: List of hidden units per layer.
        :type n_classes: int.
        """
        global _loss
        super(DNNClassifier, self).__init__()
        self.feature_layer = None
        self.n_classes = n_classes
        if feature_columns is not None:
            # combines all the data as a dense tensor
            self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)
        self.hidden_layers = []
        for hidden_unit in hidden_units:
            self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit, activation='relu'))
        if self.n_classes == 2:
            # special setup for binary classification
            pred_act = 'sigmoid'
            _loss = 'binary_crossentropy'
            n_out = 1
        else:
            pred_act = 'softmax'
            _loss = 'categorical_crossentropy'
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

def optimizer(learning_rate=0.001):
    """Default optimizer name. Used in model.compile."""
    return tf.keras.optimizers.Adagrad(lr=learning_rate)

def loss(labels, output):
    """Default loss function. Used in model.compile."""
    global _loss
    if _loss == "binary_crossentropy":
        return tf.reduce_mean(tf.keras.losses.binary_crossentropy(labels, output))
    elif _loss == "categorical_crossentropy":
        return tf.reduce_mean(tf.keras.losses.sparse_categorical_crossentropy(labels, output))

def prepare_prediction_column(prediction):
    """Return the class label of highest probability."""
    return prediction.argmax(axis=-1)

def eval_metrics_fn():
    return {
        "accuracy": lambda labels, predictions: tf.equal(
            tf.argmax(predictions, 1, output_type=tf.int32),
            tf.cast(tf.reshape(labels, [-1]), tf.int32),
        )
    }
`
