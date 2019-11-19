# Couler and SQLFlow

This design is about the migration from SQLFlow submitters to use Couler.

Couler is a compiler that translates a workflow represented by a Python program into an Argo YAML file, which can run on Argo, the Kubernetes-native workflow execution engine. Couler is also a framework that directs users to define steps and workflows as Python functions.

SQLFlow is a compiler that translates a SQL program into a Python program known as a *submitter*. Currently, SQLFlow has several compiler backends known as *codegen*s.  For example, `codegen_xgboost.go`  generates a submitter that calls ODPS for the execution of usual SQL queries and XGBoost for training and prediction models.  The Python code that calls ODPS and XGBoost deposits in the `python/` directory of the SQLFlow source code repository.

The migration includes the following parts:

1. Converts Python source code called by submitters into Couler definitions. For example:
   - `couler.{odps,mysql,hive}.query(sql)` run a SQL program/statement on ODPS/MySQL/Hive
   - `couler.{odps,mysql,hive}.export(table, filename)` exports a table from ODPS/MySQL/Hive to RecordIO files
   - `couler.{xgboost,tensorflow,elasticdl}.train(model_def, data)` trains an XGBoost/TensorFlow/ElasticDL model
   - `couler.{xgboost,tensorflow,elasticdl}.predict(trained_model, data)` predicts using an XGBoost/TensorFlow/ElasticDL model

1. Deposits some frequently reusable workflows into Couler functions. For example:
   - `sqlflow.couler.query(db_info, sql)` calls `couler.{odps,mysql,hive}.query(sql).
   - `sqlflow.couler.{xgboost,tensorflow,elasticdl}.train(train_ir)` calls `sqlflow.couler.query`, `sqlflow.couler.export`, and then `couler.{xgboost,tensorflow,elasticdl}.train(...)`.

1. Instead of having multiple codegens, let us have only one, `codegen_couler.go`, which translates the [Intermediate Representation](/doc/design/intermediate_representation.md)(IR) of a SQL program into a Couler program. Then, SQLFlow can run the Couler compiler to convert further and execute the workflow.

For example, `codegen_couler.go` converts a `SELECT ... TO TRAIN` statement into the call to `sqlflow.couler.{xgboost,tensorflow.elasticdl}.train(...)`.

## Couler Step Function

For example, an extended SQL `SELECT ... TO TRAIN xgboost.booster ...` would be compiled into a
an Couler step function by `codegen_couler.go`:

``` python
couler.sqlflow.run('echo "SELECT ... TO TRAIN xgboost.booster" | sqlflow -parse | python -m sqlflow_submitter.xgboost.train', image="sqlflow/sqlflow_submitter")
```

In the above Couler function:

- `sqlflow -parse` is a command-line tool which parses an extended SQL into an SQLFlow IR with JSON format, like:

    ``` json
    {
        "dataSource": "mysql://user:pass@192.168.1.1:3306",
        "select": "SELECT * FROM iris.train",
        "validation_select": "SELECT * FROM iris.test",
        "estimator": "XGBOOST.gbtree",
        "attributes": {"train.num_boost_round": 30},
        "features": {"sepal_length": {"type": "numeric", "shape": [1], "field_meta":{"name":"sepal_length", "dtype": "float32", "delimiter": "", "is_sparse": False}}...},
        "label": {"class": {"type": "numeric", "shape": [1], "field_meta": ...}}
    }
    ```

- `sqlflow_submitter.xgboost.train` is a Python module that submits an XGboost training job according to the input IR structure.
- `sqlflow/sqlflow_submitter` is the SQLFlow submitter Docker image which packages:
    1. the SQLFlow command-line tool to compile the SQL statement into IR structure with JSON format.
    1. the SQLFlow submitter Python package under `python/sqlflow_submitter` directory.

### Couler Step Function and Model Zoo

For the custom model in [Model Zoo](/doc/design/model_zoo.md), each model would be packaged into a Docker image and
users can specify this Dockera image in SQL:  `SELECT ... TO TRAIN regressors:v0.2/MyDNNRegressor`, the Couler step function can be like:

``` python
couler.sqlfow.run('echo "SELECT ... TO TRAIN ... | sqlflow -parse | python -m sqlflow_submitter.tensorflow.train"', image="regressors:v0.2/MyDNNRegressor")
```

The above customed model Docker image should base on `sqlflow/sqlflow_submitter`. Users can also launch the custom model Docker container on host, it's easy to debug with SQLFlow:

``` bash
> docker run --rm -it -v$PWD:/models regressors:v0.2/MyDNNRegressor bash
> sqlflow -parse < a.sql > ir.json
> python -m sqlflow_submitter.tensorlfow.train < ir.json
```
