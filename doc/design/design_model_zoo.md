# Model Zoo

SQLFlow model zoo is a place to store model definitions, pre-trained model weights and model documentations. You can directly train, predict, analyze using one of the models using SQLFlow, or you can do model fine-tune, transfer learning to use the model to fit your own dataset.

SQLFlow should support below features to support common cases in machine learning:

1. Host model defination and pre-trained weights in `sqlflow.org`. e.g. `sqlflow.org/modelzoo/iris_dnn_128x32` points to a directory containing a model defination of `DNNClassifier` with 128, 32 hidden layers and pre-trained weights using the iris dataset.
1. Download pre-trained model to predict a dataset:
   ```sql
   SELECT * FROM iris.predict_samples
   PREDICT predict_result.class
   USING sqlflow.org/modelzoo/iris_dnn_128x32;
   ```
1. Train a new model using the model defination:
   ```sql
   SELECT * FROM iris.new_iris_train
   TRAIN sqlflow.org/modelzoo/iris_dnn_128x32
   INTO modeldb.my_iris_dnn_model;
   ```
1. Transfer learning to fit a new dataset with the same features:
   ```sql
   SELECT * FROM iris.transfer_iris_samples
   TRAIN sqlflow.org/modelzoo/iris_dnn_128x32
   USING sqlflow.org/modelzoo/iris_dnn_128x32
   INTO modeldb.my_iris_dnn_model;
   ```
1. Fine-tune the pre-trained model:
   ```sql
   SELECT * FROM iris.train
   TRAIN sqlflow.org/modelzoo/iris_dnn_128x32
   WITH model.learning_rate=0.001, model.learning_rate_decay="cosine_decay" ...
   INTO modeldb.my_iris_dnn_model_fine_tune;
   ```

## The Model Zoo Hosting Service

The model zoo hosting service is a file service that can be accessed from the internet.
It serves all available models and corresponding pre-trained weight. We may need to use
a [CDN](https://en.wikipedia.org/wiki/Content_delivery_network) service if the traffic
becomes large.

Each model is saved on the server under a unique directory like: `sqlflow.org/modelzoo/iris_dnn_128x32`.
The directory name is responsible to explain the model's type, network structure and which dataset is
used to train the pre-trained weights. You can access `sqlflow.org/modelzoo/iris_dnn_128x32/README.md`
from the browser to get the model's full documentation. All models under `sqlflow.org/modelzoo` are
developed under `https://github.com/sql-machine-learning/models` weights is only stored under
`sqlflow.org` but not under github.

The content of the directory should be like:

```
iris_dnn_128x32
   - README.md     # model documents.
   - model_def.py  # python file of model definition (Tensorflow Estimator model or Keras model).
   - model/        # the saved Tensorflow/Keras/XGBoost model.
   - columns       # saved codegen.FeatureColumn meta for parsing and extracting features.
   - some_deps.py  # if model_def.py have dependent python source files just put them in the same folder.
```

- Note that we save the `columns` file so that we can check if user-provided data is of the same type.
- Note that we only support Tensorflow Estimator model, Keras model, and XGBoost model currently, and the
`model_def.py` file is the model python definition only when the model is a Tensorflow Estimator model or Keras model. For XGBoost model, the content of `model_def.py` should be like: `model_type = xgboost.gbtree`.

To publish a new model into the model zoo, you must:

1. commit your model (code and README.md) to https://github.com/sql-machine-learning/models and merge the code.
1. train and test your model on the dataset then save the pre-trained model together with `columns` file.
1. name a directory and upload all files listed above into the directory on the server.

## Using the Model Zoo in SQLFlow Statements

In SQLFlow, you can specify the model under `sqlflow.org/modelzoo` in the `TRAIN` and `USING` clause.

If the `TRAIN` clause accepts a model under `sqlflow.org/modelzoo`, SQLFlow will only use the model definition to start the train. If `USING` clause accepts a model under `sqlflow.org/modelzoo`, the model's
weights will be loaded both in `TRAIN` process or `PREDICT` process.

For models that supports load only parts of the weights for transfer learning or prediction, the layers
in the model should have different name if you do not want to load the weights for current layer when
transfer learning (refer to [Define Models for SQLFlow](desing_customized_model.md) for how to implement a customized model):

```python
class DNNClassifier(tf.keras.Model):
    def __init__(self, feature_columns, hidden_units=[10,10], n_classes=2, is_transfer=False):
        super(DNNClassifier, self).__init__()

        # combines all the data as a dense tensor
        self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)
        self.hidden_layers = []
        for idx, hidden_unit in enumerate(hidden_units):
            if is_transfer:
                dense_name = "dense_trans_%d" % idx
            self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit, name=dense_name))
        if is_transfer:
            dense_name_out = "dense_out_transfer"
        self.prediction_layer = tf.keras.layers.Dense(n_classes, activation='softmax', name=dense_name_out)
```

When training or predicting, the user-provided columns may be different from the model in the model zoo,
the training or prediction procedure should throw an error immediately.