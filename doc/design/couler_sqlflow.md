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

For example, an XGBoost train step can be like:

``` python
couler.run_container(
    cmd='''
echo "SELECT ... TO TRAIN xgboost.booster" |
sqlflow -parse |
python -m runtime.xgboost.train
''',
    env={"SQLFLOW_DATASOURCE": "mysql://user:pass@192.168.1.1:3306"}, # set session message as the env vars.
    image="sqlflow/sqlflow:step"
)
```

From the above Couler function:
- `sqlflow -parse` is **SQLFLow command-line tool**, which compiles the input extended SQL into an IR serialized file.
- `runtime.xgboost.train` is **SQLFlow submitter Python module**, which accepts the IR protobuf text file and then submit a machine learning job.
- `sqlflow/sqlflow:step` is a Docker image that packages the SQLFlow `runtime` Python module and SQLFlow command-line tool.

### SQLFLow Command-line Tool

`sqlflow -parse` is a command-line tool which accepts an extended SQL and outputs the SQLFlow IR with
protobuf text format, the protobuf definition is as follows:

```protobuf
message FieldDesc {
    required string name = 1;
    required FieldType dtype = 2;
    optional string delimiter = 3;
    repeated int shap = 4;
    required bool is_sparse = 5;
    required map<string, string> vocabulary = 6;
    required int32 max_id = 7;
}

message NumericColumn {
    FieldDesc field_meta = 1;
}

message BucketColumn {
    ...
}

message FeatureColumn {
    oneof feature_column {
        NumericColumn nc = 1;
        BucketColumn bc = 2;
        CrossColumn cc = 3;
        ...
    }
}

message Session {
    string token = 1;
    string db_conn_str = 2;
    bool exit_on_submit = 3;
    string user_id = 4;
    // for loading CSV to hive
    string hive_location = 5;
    string hdfs_namenode_addr = 6;
    string hdfs_user = 7;
    string hdfs_pass = 8;
}

message IR {
    required string datasource = 1;
    required string select = 2;
    optional string validation_select  = 3;
    required string estimator = 4;
    optional map<string, string> attributes = 5;
    optional map<string, FeatureColumn> features = 6;
    optional FeatureColumn label = 7;
    optional Session session = 8;
}
```

Note: You can check more details about the IR definition from [ir.go](/pkg/ir/ir.go).

### SQLFLow Submitter Python Module

An SQLFlow submitter Python module `runtime.{tensorflow,xgboost,elasticdl}.train` accepts an SQLFlow IR with protobuf text format, and then submit a TensorFlow, XGBoost or ElasticDL training job, we can call it like:

``` bash
cat ir.proto_text | python -m runtime.xgboost.train
```

### Couler Step Function and Model Zoo

For a custom model in [Model Zoo](/doc/design/model_zoo.md), each model would be packaged into a Docker image and
users can specify this Docker image in SQL:  `SELECT ... TO TRAIN regressors:v0.2/MyDNNRegressor ...`, the Couler step function can be like:

``` python
couler.sqlflow.run('''
echo "SELECT ... TO TRAIN regressors:v0.2/MyDNNRegressor ..." |
sqlflow -parse |
python -m runtime.xgboost.train
''',
    env={"SQLFLOW_DATASOURCE": "mysql://user:pass@192.168.1.1:3306"}, # set session message as the env vars.
    image="regressors:v0.2")
```

The above customized model Docker image should base on `sqlflow/sqlflow:step`. Users can also launch the custom model Docker container on host, it's easy to debug with SQLFlow:

``` bash
> docker run --rm -it -v$PWD:/models regressors:v0.2/MyDNNRegressor bash
> sqlflow -parse < a.sql > ir.json
> python -m runtime.tensorflow.train < ir.json
```
