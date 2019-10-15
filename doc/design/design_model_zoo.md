# Model Zoo

SQLFlow model zoo is a place to store model definitions, pre-trained model parameters (or weights) and model documentations. You can directly train, predict, analyze using one of the models using SQLFlow, or you can do model fine-tune, transfer learning to use the model to fit your dataset.

SQLFlow should support below features to support common cases in machine learning:

1. Host model definition and pre-trained parameters in `sqlflow.org`. e.g. `sqlflow.org/modelzoo/iris_dnn_128x32/v1.0` points to a directory containing a model definition of `DNNClassifier` with 128, 32 hidden layers and pre-trained parameters using the iris dataset.
1. Download pre-trained model to predict a dataset:
   ```sql
   SELECT * FROM iris.predict_samples
   PREDICT predict_result.class
   USING sqlflow.org/modelzoo/iris_dnn_128x32/v1.0;
   ```
1. Train a model from scratch using the model definition:
   ```sql
   SELECT * FROM iris.new_iris_train
   TRAIN sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
   INTO modeldb.my_iris_dnn_model;
   ```
1. [Transfer learning](https://en.wikipedia.org/wiki/Transfer_learning) to fit a new dataset with the same features:
   ```sql
   SELECT * FROM iris.transfer_iris_samples
   TRAIN sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
   USING sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
   INTO modeldb.my_iris_dnn_model;
   ```

## Concept of A Model in the Model Zoo

A Model in the Model Zoo contains below components:

1. The model definition. A python program defines the model's structure, currently it can be:
    - TensorFlow Estimator
    - A Keras subclass model
    - A XGBoost model type
1. Pre-trained model parameters.
1. Model publication information, including:
    - Unique model name
    - Author
    - Version
    - Hyper Parameters used to train the model parameters
    - How to convert database input to model input

Note that the model definition may have configurable hyper-parameters for training, like the size
and depth of the DNN layers, loss function and optimizer used to train the model. We can train
many different model parameters with different hyper parameter settings use one model definition.
Also we can train the model parameters using different dataset. So the model definition can be reused
to form many "models" to solve the real-world problem, but the dataset, hyper-parameters, model parameters
are unique for each "model" to solve a problem.

The "model" in the model zoo does not mean a model definition here, but a "model" with model parameters
and other information used to solve one real-world problem. So we always use the dataset name, model name,
model structure description to form a "model name" to indicate that this model is used to solve this kind
of problem.

## The Model Zoo Hosting Service

The model zoo hosting service is a file service that can be accessed from the internet.
It serves all available models and corresponding pre-trained weight. We may need to use
a [CDN](https://en.wikipedia.org/wiki/Content_delivery_network) service if the traffic
becomes large.

The publication of a trained model needs to upload the parameters of the model to a
storage service, labeled with the version, or Git commit ID, of the model zoo.
The model directory is of the format: `sqlflow.org/modelzoo/iris_dnn_128x32/v1.0/`.
`iris_dnn_128x32` is the model's name, and `v1.0` is the version.
The directory name is responsible to explain the model's type, network structure and which dataset is
used to train the pre-trained parameters. You can access `sqlflow.org/modelzoo/v1.0/README.md`
from the browser to get the model's full documentation. All models under `sqlflow.org/modelzoo` are
developed under `https://github.com/sql-machine-learning/models` parameters is only stored under
`sqlflow.org` but not under Github.

The content of the directory should be like:

```
iris_dnn_128x32/v1.0/
   - model_meta.json  # model information useful for load and run.
   - README.md        # model documents.
   - requirements.txt  # python package dependency for the model.
   - model_def.py     # python file of model definition (TensorFlow Estimator model or Keras model).
   - model/           # the saved TensorFlow/Keras/XGBoost model.
   - some_deps.py     # if model_def.py have dependent python source files just put them in the same folder.
```

Some details about the files in one model:

- `model_meta.json` contains important information used for load and run this model, a sample is shown below:
    ```json
    {
        // engine used to train this model, can be TensorFlow, Keras or XGBoost.
        // and the version of the engine used.
        "model": {
            "engine": "tensorflow",
            "version": "2.0.0",
        },
        // SQL statement used when train the model, this is useful when you use this
        // model to run prediction using sqlflow. Things can be extracted from the SQL:
        // 1. The model name (a python class name defined in "model_def.py")
        // 2. How to extract column data (FieldMeta info from DENSE/SPARSE syntax)
        // 3. Target to train or predict
        "train_sql": {
            "SELECT * FROM traintable TRAIN ...",
        },
        // The SQL statement may not have full specification of how to parse the column
        // data, the "columns" section contains derivated FieldMetas when training. We
        // use information in "columns" section to construct "FieldMeta" when using this
        // model. If the input data does not fit the definitions under "columns" section,
        // the error will be raised.
        "columns": {
            "col1": {
                "Shape": [1],
                "IsSparse": false,
                "Delimiter": "",
                "DType": "Float",
            },
            "col2": {...},
        },
        "label": {
            "Shape": [1],
                "IsSparse": false,
                "Delimiter": "",
                "DType": "Int",
        },
    }
   ```
- `model_def.py` file is the model python definition. The content varies when using different engines:
    - Custom Estimator: A sub class of `tf.estimator.Estimator`
    - Keras Model: A keras sub class model definition.
    - XGBoost Model: One line indicating the XGBoost supported model type, like: `model_type = xgboost.gbtree`

## ElasticDL Compatible Model

On the one hand, the model trained by ElasticDL can also be published in SQLFlow model zoo. A `model_meta.json` file will be generated when training ElasticDL.

On the other hand, If you want to train/fine-tune a model from the model zoo with ElasticDL, SQLFlow can use the "columns" information in `model_meta.json` to form a `dataset_fn` which ElasticDL needed.
see: https://github.com/sql-machine-learning/sqlflow/blob/develop/pkg/sql/template_elasticdl.go#L46

## Publish A Model to the Model Zoo

To publish a new model into the model zoo, you need to:

1. commit your model (code and README.md) to https://github.com/sql-machine-learning/models and merge the code. A model in the models repo should contain below files:
    ```
    - README.md
    - requirements.txt
    - model_def.py
    - some_deps.py
    ```
1. train and test your model on the dataset then save the model parameters together with `model_meta.json` file.
1. name a directory and upload all files listed above into the directory on the server.

The overall workflow to publish and use a model in the model zoo is shown below:

<p align="center">
<img src="../figures/modelzoo_workflow.png" width=500px>
</p>

## Model Sharing

Model sharing is a necessary feature to encourage more users to contribute models to the model zoo. The
features are quite common for products like [DockerHub](https://hub.docker.com/). We can discuss these
features when we need to implement the model sharing features.

## Use the Model Zoo in SQLFlow Statements

In SQLFlow, you can specify the model under `sqlflow.org/modelzoo` in the `TRAIN` and `USING` clause.

If the `TRAIN` clause accepts a model under `sqlflow.org/modelzoo`, SQLFlow will only use the model definition to start the train. If `USING` clause accepts a model under `sqlflow.org/modelzoo`, the model's
parameters will be loaded both in `TRAIN` process or `PREDICT` process.

Supported use cases:

1. Predict some data using a pre-trained model:
    ```sql
    SELECT * FROM iris.predict_samples
    PREDICT predict_result.class
    USING sqlflow.org/modelzoo/iris_dnn_128x32/v1.0;
    ```
    By using the SQL statement above, SQLFlow will download all the contents under
    `sqlflow.org/modelzoo/iris_dnn_128x32/v1.0`, then use the data from table `iris.predict_samples`
    as input, then output the predict result on the screen.
1. Transfer learning to fit your data:
    ```sql
    SELECT * FROM iris.transfer_iris_samples
    TRAIN sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
    USING sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
    INTO modeldb.my_iris_dnn_model;
    ```
    SQLFlow will first load the model definition and parameters using
    `sqlflow.org/modelzoo/iris_dnn_128x32/v1.0`, then train the model using the data in
    `iris.transfer_iris_samples`. The model will use the basic "knowledge" from our
    pre-trained parameters and learn to fit your data fast, the trained model is saved
    into the table `modeldb.my_iris_dnn_model`.
1. Train a model from scratch:
    ```sql
    SELECT * FROM iris.new_iris_train
    TRAIN sqlflow.org/modelzoo/iris_dnn_128x32/v1.0
    INTO modeldb.my_iris_dnn_model;
    ```
    By using the SQL statement above, SQLFlow will **not** load the model parameters,
    it will initialize the parameters randomly and train the model only using the model
    definition from `sqlflow.org/modelzoo/iris_dnn_128x32/v1.0`, the trained model is saved
    into the table `modeldb.my_iris_dnn_model`.

For models that supports load only parts of the parameters for transfer learning or prediction, the layers
in the model should have different name if you do not want to load the parameters for current layer when
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