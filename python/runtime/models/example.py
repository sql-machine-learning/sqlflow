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

import random

import pandas as pd
import tensorflow as tf
from runtime.tensorflow.load_model import load_keras_model_weights
from sklearn.model_selection import train_test_split

data = {
    'c1': [random.random() for _ in range(300)],
    'c2': [random.random() for _ in range(300)],
    'c3': [random.random() for _ in range(300)],
    'c4': [random.random() for _ in range(300)],
    'c5': [random.random() for _ in range(300)],
    'target': [random.randint(0, 2) for _ in range(300)]
}
dataframe = pd.DataFrame.from_dict(data)

train, test = train_test_split(dataframe, test_size=0.2)
train, val = train_test_split(train, test_size=0.2)
print(len(train), 'train examples')
print(len(val), 'validation examples')
print(len(test), 'test examples')


# A utility method to create a tf.data dataset from a Pandas Dataframe
def df_to_dataset(dataframe, shuffle=True, batch_size=1):
    dataframe = dataframe.copy()
    labels = dataframe.pop('target')
    ds = tf.data.Dataset.from_tensor_slices((dict(dataframe), labels))
    if shuffle:
        ds = ds.shuffle(buffer_size=len(dataframe))
    ds = ds.batch(batch_size)
    return ds


batch_size = 32  # A small batch sized is used for demonstration purposes
train_ds = df_to_dataset(train, batch_size=batch_size)
val_ds = df_to_dataset(val, shuffle=False, batch_size=batch_size)
test_ds = df_to_dataset(test, shuffle=False, batch_size=batch_size)

feature_columns = [
    tf.feature_column.numeric_column(header)
    for header in ['c1', 'c2', 'c3', 'c4', 'c5']
]


class DNNClassifier(tf.keras.Model):
    def __init__(self, feature_columns, hidden_units, n_classes):
        """DNNClassifier
        :param feature_columns: feature columns.
        :type feature_columns: list[tf.feature_column].
        :param hidden_units: number of hidden units.
        :type hidden_units: list[int].
        :param n_classes: List of hidden units per layer.
        :type n_classes: int.
        """
        super(DNNClassifier, self).__init__()

        # combines all the data as a dense tensor
        self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)
        self.hidden_layers = []
        for hidden_unit in hidden_units:
            self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit))
        self.prediction_layer = tf.keras.layers.Dense(n_classes,
                                                      activation='softmax')

    def call(self, inputs):
        x = self.feature_layer(inputs)
        for hidden_layer in self.hidden_layers:
            x = hidden_layer(x)
        return self.prediction_layer(x)

    def default_optimizer(self):
        """Default optimizer name. Used in model.compile."""
        return 'adam'

    def default_loss(self):
        """Default loss function. Used in model.compile."""
        return 'categorical_crossentropy'

    def default_training_epochs(self):
        """Default training epochs. Used in model.fit."""
        return 5

    def prepare_prediction_column(self, prediction):
        """Return the class label of highest probability."""
        return prediction.argmax(axis=-1)


model = DNNClassifier(feature_columns=feature_columns,
                      hidden_units=[10, 10],
                      n_classes=3)
is_training = False
if is_training:
    model.compile(optimizer=model.default_optimizer(),
                  loss=model.default_loss())
    model.fit(train_ds,
              validation_data=val_ds,
              epochs=model.default_training_epochs(),
              verbose=0)
    model.save('my_model', save_format="tf")
    print("Done training.")
else:
    model.predict(test_ds)
    load_keras_model_weights(model, 'my_model')
    prediction = model.predict(test_ds)
    print(model.prepare_prediction_column(prediction))
    print("Done predicting.")
