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

For example, `codegen_couler.go` converts a `SELECT ... TO TRAIN` statement into the call to `sqlflow.couler.{xgboost,tensorflow.elasticdl}.train(train_ir)`, which `train_ir` is the SQLFlow IR with JSON format.
