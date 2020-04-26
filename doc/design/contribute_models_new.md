# Steps to Contribute a Model to SQLFlow

[This document](../contribute_models.md) explains how a ML specialist develops, tests and publishes a model definition. In this design, we want to present a completely new way that tremendously simplize the steps.

## Prepare

Run the below command to install SQLFlow Python bindings.

```bash
pip install sqlflow
```

## Model Development in Python

### Define a Model

For clarity, let us consider a deep learning model definition is a Keras class in Python. For example, the following class in `some_model_definitions.py`:

```python
import tensorflow as tf

class MyAwesomeClassifier(tf.keras.Model):
   def __init__(self, ...):
      ...
```

For detailed instructions, please refer to this
[document](https://github.com/sql-machine-learning/models/blob/develop/doc/customized%2Bmodel.md).

### Test and Verify

To verify that this model definition works with SQLFlow, we can call SQLFlow's High-level API:

```python
import sqlflow

sqlflow.init("mysql://user:pass@localhost:3306")
sqlflow.train(
  "SELECT * FROM train_data_table",
  MyAwesomeClassifier,
  attrs={"param1":value, "param2":value2},
  columns={"column1":lambda(x):..., "column2":labmda(x):...},
  into="trained_model")
```

By calling `sqlflow.train` SQLFlow will run the training job locally to verify that the model works. If any error occurs you can update your code and do debugging until the model works as desired. This call works complete the same as the below SQL statement.

```sql
SELECT ... TO TRAIN MyAwesomeClassifier WITH ... COLUMNS ... INTO trained_model;
```

### Unit Tests

With the high-level API, we can test the model definition by writing a unit test file like `tests.py`. This will enable you to continuously evolve your model.

```python
import sqlflow

def test_MyAwesomeClassifier():
  # connect to local mysql setup and test.
  sqlflow.init(dbhost="mysql://127.0.0.1:3306")
  sqlflow.train("SELECT ...", MyAwesomeClassifier, ... into="my_trained_model")
  auc, f1 = sqlflow.evaluate("SELECT ...", using="my_trained_model", metric=["AUC", "F1"])
  unittest.assert(auc > 0.9)
  unittest.assert(f1 > 0.5)
```

The above exmaple connects to a local MySQL database system.  We can connect to other supported database systems as well:

```python
import sqlflow

def test_MyAwesomeClassifier():
  ## connect to remote aMaxCompute database.
  sqlflow.init(dbhost="maxcompute://[AK]:[SK]@service-corp.odps.aliyun-inc.com/api?curr_project=alifin_jtest_dev&scheme=http")
  sqlflow.train("SELECT ...", MyAwesomeClassifier, ... into="my_trained_model")
  auc, f1 = sqlflow.evaluate("SELECT ...", using="my_trained_model", metric=["AUC", "F1"])
  unittest.assert(auc > 0.9)
  unittest.assert(f1 > 0.5)
```

We can run these unit tests by typing the following command.

```bash
pytest
```

### Test Using SQLFlow Command-line Tool

Like MySQL has a command-line client `mysql`, we provide `sqlflow`.
Using this command-line tool, we can run the following command to use
the model definition.

```bash
$ sqlflow -host="192.168.1.1:50051" -dbstr="maxcompute://...."
sqlflow@192.168.1.1:50051 > SELECT * FROM iris
TO TRAIN MyKerasModel
LABEL class
INTO my_iris_model;
```

## Release Your Model to Model Zoo

If you want to make your model definition visible to other users, you can release your model definition to SQLFlow model zoo using the SQLFlow command-line tool (assume you have your model source code under direcotry `my_model_collection/`):

```bash
$ sqlflow release modeldef my_model_collection/ model_image:v0.1

checking model is valid ...[OK]
building model docker image with version v0.1 ... [OK]
model built as hub.docker.com/sqlflow_model_zoo_public/model_image:v0.1 on https://models.sqlflow.org.
Model added successfully，You can run it `TO TRAIN hub.docker.com/sqlflow_model_zoo_public/model_image:v0.1/MyKerasModel`
```

**NOTE: the Docker registry "hub.docker.com/sqlflow_model_zoo_public/" is configured by the model zoo server, see [model zoo](./model_zoo.md) design.**

After the releasing process is done, you can use the released Docker image to train/predict your model like: `SELECT ... TO TRAIN hub.docker.com/sqlflow_model_zoo_public/model_image:v0.1/MyKerasModel ...`

To remove the model definition image, you can run:

```bash
$ sqlflow drop modeldef model_image:v0.1
```

For more design details about using the SQLFlow command-line tool to release, sharing and delete both model definitions and trained models, please refer to [this document](./model_zoo.md).
