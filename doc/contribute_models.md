# Define Models in Python And Call Them From SQL

SQLFlow extends SQL syntax to do AI.  The syntax extension allow SQL statements referring to model definitions defined as Python functions and classes, for example, https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnregressor.py.

If you are not a data analyst using SQL, but a deep learning researcher who would like to create a model for data analysts.  This document is for you.

Please be aware that SQLFlow is a gRPC server, which translates a SQL program into a workflow for execution on Kubernetes.  Some steps of this workflow might submit a TensorFlow job on Kubernetes to train the referred model definition.  To make the translation possible, the SQLFlow server needs to know the Python source code of the model definition.  We typically deploy and run the SQLFlow server in Docker containers, so the model source code need to be packed into the Docker image together with the SQLFlow server.

In the future, we will make SQLFlow server able to refer to model definitions in other Docker images.  For now, let us assume you, dear deep learning researcher, know how to use Git and Docker, and know how to build a Docker image with the SQLFlow server and your model definitions.

## Define Models as Python Source Code

To build a Docker image, we need a file named `Dockerfile`.  Suppose that we put it in a directory `~/my_models/Dockerfile` -- you are feel to leave it any directory you want.  We also want to have your model definitions in the same directory, say `~/my_models/my_awesome_model/my_awesome_model.py`, so that in the Dockerfile you can write some lines to to add these model definitions into the Docker image.  The first line of Dockerfile should be `FROM sqlflow/sqlfolw`, which makes sure that Docker images built from this Dockerfile contains the SQLFlow server.

To keep track of your edit to files in this directory, you can make it a Git repository.  You can even share your repository through GitHub.  For more about Git and GitHub, please refer to related documents.  We plan to provide a command-line tool `sqlflow` to simplify the engineering process for researchers who are not familiar with Git, GitHub, or Docker. 

Here are some quick steps for researchers who would like to contribute to SQLFlow's official model repo.

1. You can contribute to SQLFlow's [model zoo repo](https://github.com/sql-machine-learning/models) by:
    1. Fork SQLFlow's model zoo repo: click "Fork" button on the right corner on page https://github.com/sql-machine-learning/models .
    1. Clone your forked repo by `git clone [your forked repo url]`, you can find the forked repo URL by clicking the green button "Clone or download".
    1. Move to the cloned directory: `cd models`.
1. Or you can create a new git repository to store your model code:
    1. Create a new repository on [github](https://github.com) or any other git systems.
    1. Move to the directory of the repository: `cd my_models` (assume you created a repo named "my_models").
    1. Create a directory under `my_models` to store Python package: `mkdir my_awesome_model`.
    
## Start a Docker Container as the Develop Environment

```bash
docker run -p 8888:8888 -v $PWD/my_awesome_model:/workspace/my_awesome_model  sqlflow/sqlflow bash -c 'export PYTHONPATH=/workspace:$PYTHONPATH; bash /start.sh'
```

Note that we set the environment variable `PYTHONPATH` so that we can directly test out the model inside this container. Change the directory to `sqlflow_models` if you are contributing models to https://github.com/sql-machine-learning/models.

## Develop In the Jupyter Notebook

Open the browser and go to http://localhost:8888, it's a Jupyter notebook environment, you can see your model development directory `my_awesome_model` together with SQLFlow's basic tutorials.

![](figures/jupyter_develop.jpg)

Click into the directory `my_awesome_model` and add the `__init__.py` and your new model file, e.g. `mydnnclassifier.py`.

In `mydnnclassifier.py` you should develop the model's Python code, typically a [Keras subclass model](https://www.tensorflow.org/guide/keras/custom_layers_and_models#the_model_class) like below:

```python
import tensorflow as tf
class MyAwesomeClassifier(tf.keras.Model):
    def __init__(self, feature_columns=None, hidden_units=[100,100]):
        """MyAwesomeClassifier
        :param feature_columns: feature columns.
        :type feature_columns: list[tf.feature_column].
        """
        super(MyAwesomeClassifier, self).__init__()
        self.feature_layer = None
        if feature_columns is not None:
            self.feature_layer = tf.keras.layers.DenseFeatures(feature_columns)
        self.hidden_layers = []
        for hidden_unit in hidden_units:
            self.hidden_layers.append(tf.keras.layers.Dense(hidden_unit, activation='relu'))
        self.prediction_layer = tf.keras.layers.Dense(1, activation='sigmoid')

    def call(self, inputs, training=True):
        if self.feature_layer is not None:
            x = self.feature_layer(inputs)
        else:
            x = tf.keras.layers.Flatten()(inputs)
        for hidden_layer in self.hidden_layers:
            x = hidden_layer(x)
        return self.prediction_layer(x)

def optimizer(learning_rate=0.001):
    return tf.keras.optimizers.Adagrad(lr=learning_rate)

def loss(labels, output):
    return tf.reduce_mean(tf.keras.losses.binary_crossentropy(labels, output))

def prepare_prediction_column(prediction):
    return prediction.argmax(axis=-1)
```

Note that we defined a class named `MyAwesomeClassifier` which will be used as the model definition. You can define whatever arguments in the `__init__` function of this class, these arguments can be used when you write the training SQL statement by adding `WITH argument=value, argument=value ...`. You also need to define three functions:

- `optimizer`: defines the default optimizer used when training.
- `loss`: define the default loss function used when training.
- `prepare_prediction_column`: define how to process the prediction output.

If you need to control the details of the training process or define custom models rather than a Keras model, you can define a function `sqlflow_train_loop` to implement custom model training processes:

```python
class MyAwesomeClassifier(tf.keras.Model):
    def __init__(self, feature_columns=None):
        ...
    def call(self, inputs, training=True):
        ...
    def sqlflow_train_loop(self, dataset, epochs=1, verbose=0):
        # do custom training here, parameter "dataset" is a tf.dataset type representing the input data.
```

In `__init__.py` you should expose your model classes by adding lines like `from mydnnclassifier import MyAwesomeClassifier`. After this, you can test your model following the below step.

## Testing and Debugging

Go back to http://localhost:8888, add an `ipynb` file to test the model by clicking the button "New" -> "Python 3"

![](figures/jupyter_create_ipynb.jpg)

Write an SQLFlow statement to test the model using iris dataset (you need to import the dataset to MySQL if you want to test the model using other datasets.), assume you have developed a model class name:

```sql
%%sqlflow
SELECT * FROM iris.train
TO TRAIN my_awesome_model.MyAwesomeClassifier
WITH model.n_classes=3
LABEL class
INTO models_db.awesome_model;
```

you may go back to `mydnnclassifier.py` and modify the model code until it works as you expected.

## Publish Your Model

In the final step, you need to publish your model so that other SQLFlow users can get the model and use it.

1. If you are contributing to https://github.com/sql-machine-learning/models, file a pull request on Github to merge your code to SQLFlow's models repo. The model should be available when SQLFlow's Docker image `sqlflow/sqlflow` is updated.
1. If you are creating your own repo, you need to write a `Dockerfile` to build your model into a Docker image:
    1. Write a `Dockerfile` like below:
    ```docker
    FROM sqlflow/sqlflow
    ADD my_awesome_model/ /models/
    ```
    1. Then build and push the Docker image by:
    ```
    docker build -t your-registry.com/model_image .
    docker push your-registry.com/model_image
    ```
    1. Then use the model image in SQLFlow by adding the Docker image name before the model name:
    **NOTE: Below statement will not work in non-workflow mode (e.g. local run), this feature will be supported in the future.**

    ```sql
    SELECT * FROM iris.train
    TO TRAIN your-registry.com/model_image/MyAwesomeClassifier
    WITH model.n_classes=3
    LABEL class
    INTO models_db.awesome_model;
    ```
