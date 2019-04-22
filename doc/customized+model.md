# Customized Model

SQLFlow supports training and predicting using customized models. This documentation explains the design choice and provides a concrete example of adding a new model.

## Keras over Estimator

We choose Keras over Estimator for the following reasons:

1. TensorFlow 2.x will closely integrate with Keras. [ref](https://www.youtube.com/watch?v=k5c-vg4rjBw)

2. Keras provides more documentation in writing customized models than estimators. For customized estimators, I've only found two examples:

   1. `DNNClassifier` that uses `core_layers.Dense`. [ref](<https://github.com/tensorflow/estimator/blob/master/tensorflow_estimator/python/estimator/canned/dnn.py#L200-L226>)
   2. `DNNClassifier` that uses `tf.layers.dense`. [ref](https://github.com/tensorflow/models/blob/master/samples/core/get_started/custom_estimator.py#L29)

   None of these is suitable for long term development.

3. There are plenty off-the-shelf model repositories written in Keras. To name a few: [ResNet](https://github.com/raghakot/keras-resnet), [Transformer](https://github.com/Lsdefine/attention-is-all-you-need-keras), [Mask  R-CNN](https://github.com/matterport/Mask_RCNN).

## Keras Model API

Keras provides three major ways to define models:

- Subclassing `tf.keras.Model`

  ```python
  class DNNClassifier(tf.keras.Model):
      def __init__(self, feature_columns, hidden_units, n_classes):
          super(DNNClassifier, self).__init__()
          self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)
          self.hidden_layers = []
          for hidden_unit in hidden_units:
              self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit))
          self.prediction_layer = tf.keras.layers.Dense(n_classes, activation='softmax')
  
      def call(self, inputs):
          x = self.feature_layer(inputs)
          for hidden_layer in self.hidden_layers:
              x = hidden_layer(x)
          return self.prediction_layer(x)
  
  model = DNNClassifier(feature_columns, hidden_units, n_classes)
  ```

  Please be aware that models subclassing from `tf.keras.Model` , only `save_weights` and `load_weights` are supported. [ref1](https://stackoverflow.com/questions/51806852/cant-save-custom-subclassed-model), [ref2](https://stackoverflow.com/questions/52826134/keras-model-subclassing-examples).

- Functional API

  ```python
  x = tf.feature_column.input_layer(shape=(5,))
  for n in hidden_units:
      x = tf.keras.layers.Dense(n, activation='relu')(x)
  pred = tf.keras.layers.Dense(n_classes, activation='softmax')(x)
  model = tf.keras.models.Model(inputs=feature_columns, outputs=pred)
  ```

  Please be aware that functional API doesn't support feature column as input, not even densed tensors generated from `tf.keras.layers.DenseFeatures(feature_columns)`. [ref1](https://github.com/tensorflow/tensorflow/issues/27416), [ref2](https://stackoverflow.com/questions/54375298/how-to-use-tensorflow-feature-columns-as-input-to-a-keras-model).

- Sequential

  ```python
  model = tf.keras.Sequential()
  model.add(tf.keras.layers.DenseFeatures(feature_columns))
  for n in hidden_units:
    model.add(tf.keras.layers.Dense(n, activation='relu'))
  model.add(tf.keras.layers.Dense(n_classes, activation='softmax'))
  ```

  Please be aware that  `tf.keras.Sequential()` only covers a small variety of models. To name a few models that are not covered: ResNet, Transforms, WideAndDeep.

The following table summarizes the pros and cons of these three methods.

| Keras Model Mode          | Feature Column as Input | Save/Load Model                                 | Model Coverage |
| ------------------------- | ----------------------- | ----------------------------------------------- | -------------- |
| SubClass `tf.keras.Model` | ☑️                       | only supports `save_weights` adn `load_weights` | High           |
| Functional API            | ❌                       | ☑️                                               | High           |
| Sequential Model          | ☑️                       | ☑️                                               | Low            |

We chose the method of subclassing `tf.keras.Model` due to its feature column support and high coverage of models.

## Creating customized models

A model is a Python class derived from `tf.keras.Model`. For example, if we want to define a `DNNClassifier` that contains several hidden layers, we can write the following.

```python
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
        self.prediction_layer = tf.keras.layers.Dense(n_classes, activation='softmax')

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
```

## Further Reading

1. Understanding Keras source code: [model.py](https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/keras/models.py), [network.py](https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/keras/engine/network.py), [training.py](https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/keras/engine/training.py).