# Design Doc: How to Define Deep Learning Models for SQLFlow

## Configure Development Environment

1. Install [Docker Community Edition](https://docs.docker.com/install/)

2. Install SQLFlow as a Python package

   ```bash
   pip install sqlflow
   ```

3. Start SQLFlow server as a container running locally

   ```bash
   docker run --rm -it \
     -p 3306:3306 \
     -p 8888:8888 \
     -e SQLFLOW_MYSQL_HOST=0.0.0.0 \
     sqlflow/sqlflow:latest \
     bash /start.sh
   ```

   The above command exposes MySQL server port 3306 out from the
   SQLFlow server container, so we can use Navicat or VSCode MySQL
   extension to manage data in the container.  It also exposes the
   Jupyter Notebook port 8888, so users can run SQLFlow programs in
   Jupyter Web page.

Companies including Ant Financial and DiDi deployed SQLFlow and
integrated into the corp data infrastructure.  Developers in these
companies don't need to install any software, but can use the Web GUI
of their data infrastructure.

## Model Development in Python

### Define a Model

For clarify, let us consider a deep learning model definition is a
Keras class in Python.  For example, the following class in
`my_keras_model.py`:

```python
import sqlflow

class MyKerasModel(tf.keras.Model):
   def __init__(self, ...):
      ...
```

For detailed instructions, please refer to this
[document](https://github.com/sql-machine-learning/models/blob/develop/doc/customized%2Bmodel.md).

### Test and Verify

To verify that this model deifniton works, we can call Keras
high-level API like `fit` to train a model and then evaluate it.

After that, we want to verify it works with SQLFlow.  This depends on
the SQLFlow high-level Python API, which is a Python abstraction of
the SQL syntax extension introduced by SQLFlow.

### SQLFlow High-level API

For example, in SQLFlow we can write

```sql
SELECT ... TO TRAIN model_def WITH ... COLUMNS ... INTO trained_model;
```

Accordingly, in Python, we can write

```python
import sqlflow

sqlflow.train(
  "SELECT ...",
  model_def,
  with={"param1":value, "param2":value2},
  columns={"column1":lambda(x):..., "column2":labmda(x):...},
  into="trained_model")
```

### Unit Tests

With the high-level API, we can test the model definition by writing a
unit test file like `test_my_keras_model.py`.

```python
import sqlflow

def test_MyKerasModel():
  ## 初始化。dbhost：本地数据库地址；AI引擎默认是本地 SQLFlow server 中的TensorFlow, XGBoost等
  sqlflow.init(dbhost="mysql://127.0.0.1:3306")
  sqlflow.train("SELECT ...", MyKerasModel, ... into="my_trained_model")
  auc, f1 = sqlflow.evaluate("SELECT ...", using="my_trained_model", metric=["AUC", "F1"])
  unittest.assert(auc > 0.9)
  unittest.assert(f1 > 0.5)
```

The above exmaple connects to a MySQL database system.  We can connect
to other supported database systems as well.

```python
import sqlflow

def test_MyKerasModel():
  ## 初始化SQLFlow。 project： maxcompute project；AI引擎默认是PAI
  sqlflow.init(dbhost="maxcompute://[AK]:[SK]@service-corp.odps.aliyun-inc.com/api?curr_project=alifin_jtest_dev&scheme=http")
  sqlflow.train("SELECT ...", MyKerasModel, ... into="my_trained_model")
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
$ sqlflow
sqlflow > SELECT * from iris
TO TRAIN MyKerasModel
LABEL class
INTO my_iris_model;
```

## Submit Models


### Submit Models

```bash
$ sqlflow
sqlflow > ADD MODELDEF def_MyKerasModel.py
checking model is valid ...[OK]
building model docker image with version v0.12 ... [OK]
model myKerasModel.py built as hub.docker.com/username/mykerasmodel:v0.12 on localhost:50051
Model added successfully，You can run it `TO TRAIN hub.docker.com/username/mykerasmodel:v0.12/MyKerasModel`
```

### Submit to various SQLFlow servers

1. To a local SQLFlow server.

   ```text
   $ sqlflow
     sqlflow@localhost:50051 >
   ```

2. To specified SQLFlow server, for example `192.168.1.1:50051`.

   ```text
   $ sqlflow -host="192.168.1.1:50051" -dbstr="maxcompute://...."
   ```

3. Using a configuration file.

   ```text
   vim ~/.sqlflow/config
   [sqlflow]
   host=192.168.1.1:50051
   dbstr=maxcompute://....
   user=username
   password=12345
   ```

### Model Versioning

Everytime we run `ADD MODELDEF`, the system generates a new version:

```text
$ sqlflow
sqlflow > ADD MODELDEF MyKerasModel.py
model MyKerasModel.py already exists, add a new version 0.13.
checking model is valid ...[OK]
building model docker image with version v0.13 ... [OK]
model myKerasModel.py built as hub.docker.com/username/mykerasmodel:v0.13 on localhost:50051
Model added successfully，You can run it `TO TRAIN hub.docker.com/username/mykerasmodel:v0.13/MyKerasModel`
```

### Manage Model Definitions

```text
$ sqlflow
sqlflow > SHOW MODELDEFS;
sqlflow > SHARE MODELDEF hub.docker.com/username/mykerasmodel:v0.13 TO xiongmu.wy;
sqlflow > DROP MODELDEF hub.docker.com/username/mykerasmodel:v0.13;
```

### Manage Trained Models

```text
$ sqlflow
sqlflow > PUBLISH my_model;
sqlflow > SHOW TRAINEDMODELS;
sqlflow > SHARE TRAINEDMODEL my_model TO xiongmu.wy;
sqlflow > DROP TRAINEDMODEL my_model;
```
