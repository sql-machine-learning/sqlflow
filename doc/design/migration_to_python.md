# Migrating to Python

The design is about migrating SQLFlow golang code that's related to ML to python.

Language migration is difficult in common sense. The section [Flaws In The Current Architecture](#flaws-in-the-current-architecture) explains why the migration should happen. If you're already convinced that the migration is necessary, this section can be safely skipped and look over at any time.

The section [A Quick View Of The Current Architecture](#a-quick-view-of-the-current-architecture) shows the distribution of golang  *ML code* in the current architecture. That's what to be migrated to python.

The section [The Proposed Architecture](#the-proposed-architecture) illustrates an outline design about how the migration will be done.

The last two sections explain how the proposed architecture solves the problem of the current architecture, as well as the preparations to be noted.

## Flaws In The Current Architecture

For SQLFlow, migrating to python might mean rewriting lots of existing golang code in python. Why should we migrate the golang *ML code* to python? Is is necessary? Is it the right time?

After demonstrating the flaws of the current architecture in the following scenarios, the answer will be obvious.

### Scenario 1: An ML Expert Wants A New Column Type

Feature engineering is a very important part of machine learning. SQLFlow supports feature engineering via the `COLUMN` clause. The current `COLUMN` clause of SQLFlow is based on `tf.feature_column` and works only for TensorFlow. In the next version, the SQLFlow `COLUMN` clause will support more comprehensive feature columns for TensorFlow, PyTorch and XGBoost. 

Suppose the user wants a `image_embedding_column ` that converts a input image into embedding for a recommender system. Adding such a new column in the current architecture requires the following main steps:

1. The python part:
   1. Implement the column in a python file. 
2. The golang part:
   1. Add a `ImageEmbeddingColumn` struct in [pkg/ir/feature_column.go](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/ir/feature_column.go).
   2. Add a `parseImageEmbeddingColumn` function in [pkg/sql/ir_generator.go](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql/ir_generator.go).
   3. Add a `switch` statement in the function `initColumnMap` in [pkg/step/feature/derivation.go](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/step/feature/derivation.go).
   4. Add a `case` in the large `switch` statement in function `generateFeatureColumnCode` in [pkg/sql/codegen/tensorflow/codegen.go](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql/codegen/tensorflow/codegen.go). This step generates a snippet of python code with simple text substitution.
   5. Check inputs for `image_embedding_column ` to avoid possible run-time exception from the python part to ensure user experience.
3. Test and debug: a bug can be in the python file, in `pkg/ir/feature_column.go`, in `pkg/sql/ir_generator.go`, in `pkg/step/feature/derivation.go`, in `pkg/sql/codegen/tensorflow/codegen.go`, or in the python code text substitution template.

See https://github.com/sql-machine-learning/sqlflow/pull/1897 and https://github.com/sql-machine-learning/sqlflow/pull/1901 for an real-world example.

### Scenario 2: An ML Expert Wants To Contribute A New Type Of Model

There're lots of models other than DNNs that have decent performance on average-size datasets, for example, tree models like `XGBoost` and `LightGBM`. Suppose the user want's to contribute `LightGBM` to SQLFlow, this requires the following main steps:

1. The python part:
   1. Add a directory `lightgbm` in [python/sqlflow_submitter](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/python/sqlflow_submitter). 
   2. Write `train.py`, `predict.py`, `explain.py`, `evaluate.py`  in the `lightgbm` directory, each file has a function with the same name as the file, and the functions implements the main logic for SQLFlow train statement, predict statement, explain statement, and evaluate statement, respectively.
2. The golang part:
   1. Add a package `pkg/sql/codegen/lightgbm/`.
   2. Add `codegen_*.go` and `template_*.go` in the `lightgbm` directory for each type of statement. Each pair of `codegen_*.go` and `template_*.go` generates a simple python script by text substitution. The python script calls the python functions in the python part. For example, `lightgbm/codegen_train.go` and `lightgbm/template_train.go` generate a python script that calls `python/sqlflow_submitter/lightgbm/train.py`
   3. Check inputs for `lightgbm ` to avoid possible run-time exception from the python part to ensure user experience.
   4. Modify the `submitter`s in the `pkg/sql/ package` to call `pkg/sql/codegen/lightgbm` to generate the *submitter program*.
3. Test and debug: a bug can be in the python files, in the golang files, or in the python code text substitution templates.

See [python/sqlflow_submitter/xgboost](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/python/sqlflow_submitter/xgboost), [pkg/sql/codegen/xgboost](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql/codegen/xgboost), and [pkg/sql/submitter.go](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql/submitter.go) for an real-world example.

### The Flaws In A Nutshell

From the two typical scenarios, the flaws in the current architecture are obvious:

1. **Mirrored classes/functions**. Each classes/functions of ML-purpose in python has a mirror struct/function in golang, if anyone upgrades the python *ML code*, this usually implies she must upgrade the golang counterpart at the same time to make the upgrade work, and vice versa. This is a strong signal of bad extensibility, which usually is led by a design flaw.
   1. It's tedious and error-prone for SQLFlow developers to develop new ML features.
   2. Moreover, It's tedious and error-prone for model developers to contribute models: because python is the dominant language in the field of ML, ML experts are always only familiar with `python`, forcing them to develop in golang will either scare away them or lead to low quality code and painful code review. 
2. **Limited semantic checking**. There's no language-level interaction between the golang part and the python part. 
   1. Text substitution means we have to write python code as a golang string. This is hard to review and debug.
   2. In a text substituion way, because the golang part lacks knowledges about python function parameters and exceptions, SQLFlow cannot generate friendly diagnostic messages for argument errors.  SQLFlow users are always only familiar with SQL, tens of lines of python traceback is a terrible user experience.

Because SQLFlow is an ML platform, and because Python is the dominant language in the ML community, SQLFlow has to rely heavily on python and has to frequently update the fresh progress of the ML community. In fact , the **Mirrored classes/functions** flaw implied the current architecture is unfriendly to both SQLFlow developers and model developers, the **Limited semantic checking** flaw implied the current architecture is unfriendly to SQLFlow users. Therefore, the two flaws are fatal for SQLFlow.

To solve the flaws of the current architecture described above. We propose to migrate the *ML code* from golang to python for two reasons:

1. To solve the problems caused by **mirrored classes/functions**, there're only two possible options: defining the ML-purpose classes/functions only in python, or defining the classes/functions only in golang. Obviously, the second option is infeasible because all the classes/functions rely heavily on TensorFlow, XGBoost or other python ML frameworks. As a result, we should define them only in python. 
2. Now that we have to define the ML-purpose classes/functions only in python, to solve the problems caused by **limited semantic checking**, we can only check the semantics of the classes/functions in python. In this approach, because python of course have the knowledges of python function parameters and execeptions, we can give comprehensive diagnostic messages easily with the reflection mechanism provided by python.

## A Quick View Of The Current Architecture

Before diving into the migration, we should walk through the current packages to judiciously judge what would migrate to python.

### The Current Python Package

The main python *ML code* are in the python package [sqlflow_submitter](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/python/sqlflow_submitter). The main purpose of the golang *ML code* is to generate python scripts that call python functions in the `sqlflow_submitter`  package. We call these python scripts the *submitter programs*. Because `sqlflow_submitter` is already written in python. There will be little modification to this package in the migration process.

### The Current Golang Package Structure

At the moment, the packages under [sqlflow/pkg](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg) are:

1. [server](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/server) is the implementation of the SQLFlow gRPC server. The input statements from the user will be sent to the SQLFlow gRPC server. It forwards the statements to the `workflow` package and get results back. We **don't** have to migrate this package to python.

2. [proto](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/proto)  defines necessary pb messages for the `server` package. We **don't** have to migrate this package to python.

3. [workflow](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/workflow) is the downstream of the `server` package. It does a text substitution and generates a python script. The python script calls [couler](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/doc/design/couler_sqlflow.md) to generate an `Argo`  `.yaml` file that defines the workflow to be executed on Argo. We propose to migrate this package to python for two reasons:

   1. The `workflow`  package has the same design as the *ML code* described above, it has similar flaws. 
   2. In the next version of the `COLUMN` clause, the `COLUMN` clause may generate its own workflow steps to derive features or get analysis data. This process will requires some *ML code* and should be implemented in python.

4. [step](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/step) is the downstream of the `workflow` package, it's binary `cmd/step` is executed as a workflow step on Argo. The `step/feature` directory should be migrated to python because it has *ML code* that deduces feature columns from the input. 

5. [database](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/database) is a utility package that wraps the `godriver`s, i. e. [goalisa](https://github.com/sql-machine-learning/goalisa), [gomaxcompute](https://github.com/sql-machine-learning/gomaxcompute), [gohive](https://github.com/sql-machine-learning/gohive), into a set of functions for ease of use by other golang code in `sqlflow/pkg` and `sqlflow/cmd`. We don't have to rewrite the `godriver`s in python because don't depend on python and they are not *ML code* . 

   However, because `step/feature` depends on `goalisa`, and would migrate to python, the `database` package has to be accessible by both python and golang. As a result, we propose to wrap this golang package as well as the `godriver`s into a python module `_database.so` to make it both available in python and golang.

6. [parser](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/parser) contains the parser of the SQLFlow extended syntax. It is not *ML code* , so we **don't** have to migrate this package to python. 

7. [ir](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/ir). This package defines the intermediate representations of a SQLFlow statement. A SQLFlow program is a set of SQLFlow statements. Intermediate representations or `ir`s for short in this package are `struct`s that implement the `ir.SQLStatement` interface. 

   The proposed architecture will require the golang part to pass the *ir* of a SQLFlow program to the python part to execute the program. To accomplish this requirement, we can either wrap `SQLStatement` into a python module as what we proposed to do with `database` package or redefine the *ir* as a protobuf message. 

   The protobuf way is preferred because the `ir` s are actually messages rather than a module. Suppose we define messages for  `Statement`s and `Program`s in `proto/ir.proto` for ease of later discussion.

8. [verifier](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/verifier). This package checks the names and types of database table columns. It would be migrated to python because It's required by `step/feature`.

9. [model](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/model). This is a utility for saving and loading trained ML models. It should be migrated to python.

10. [pipe](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/pipe). This is a utility for piping `stdout` of a CLI process as a stream for later use. It doesn't require python and is not *ML code*, so we **don't** have to migrate this package to python. 
11. [log](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/log). This is a utility package for logging messages, for example: `logger.Info(something)`. It doesn't require python and is not *ML code*, so we **don't** have to migrate this package to python. 

13. [sql/codegen](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql/codegen). This package implements the mentioned *text substitution* to generate *submitter programs*. This package is the core reason of **mirrored classes/functions**: Every function/package in `sqlflow_submitter` has a corresponding function/package in `sql/codegen`.

    After the migration to python, we'll have a new python package that directly calls functions in `sqlflow_submitter`. Therefore, we don't need *text substitution* in the proposed architecture. We should **remove this package** after the migration, 

14. [sql](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sql) dispatches `ir`s of SQLFlow statements to appropriate platforms (PAI e.g.) and ML engines (XGBoost e.g.), and calls the corresponding `codegen` package (`codegen/xgboost` e.g.) to generate the *submitter program* that calls corresponding python functions, and spawns a process to execute the program.

    The main component of this package is a visitor hierarchy that implements a double dispatch mechanism. The hierachy is composed of `struct`s that implement the `ir.Executor` interface. 

    The proposed architecture also has to dispatch the `ir`s to the python functions in `sqlflow_submitter`, so we should reimplement a double dispatch mechanism in python for the proposed architecture. 

15. [sql/codegen/attribute](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/attribute) provides **limited semantic checking** for the python classes/functions. As discussed above, It should be migrated to python.

16. [sqlfs](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/sqlfs) wraps a database table into a file system interface and is required by the `model` package. We would migrate this package to python together with `model`.

17. [table_writer](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/table_writer) is a utility package for rendering database tables for a UI. We **don't** have to migrate this package to python. It's required by the `step` package and should be move to the `step` directory.

## The Proposed Architecture

### The Proposed Golang Package Structure

Based on the foregoing analysis, the directory structure SQLFlow core code under `sqlflow/pkg` should be:

| Package Name | Modification                                                 |
| ------------ | ------------------------------------------------------------ |
| `server`     | Calls `executor` instead of `workflow`                       |
| `proto`      | Add a new file: `ir.proto`                                   |
| `step`       | Remove `step/feature`. Move `tablewriter` to `step/tablewriter` |
| `parser`     | Stay the same.                                               |
| `ir`         | Remove `ir/feature_column.go`.                               |
| `executor`   | Rename`sql` to `executor`. Remove `sql/codegen` and `sql/codegen/attribute`. |
| `database`   | Provide a python wrapper.                                    |
| `pipe`       | Stay the same.                                               |
| `log`        | Stay the same.                                               |

### The Proposed Python Package Structure

Add a package named `sqlflow` in [sqlflow/python](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/python) to contain the python code imgrated from golang. The directory structure of SQLFlow core code under `sqlflow/python/sqlflow` should be:

| Package/Module Name | Description                                                  |
| ------------------- | ------------------------------------------------------------ |
| `platform`          | Modules under this package must implement the same set of functions(`run`, `train`, `evaluate`, `explain`, `predict` and `solve`) with the same signatures (e.g. `train(stmt: Statement)`) |
| `columns`           | Modules under this package defines functions that can be called by users in a `COLUMN` clause. |
| `features.py`       | Translate the `COLUMN` clause into calls to functions in the `column` package. |
| `execute.py`        | Execute a SQLFlow program or a SQLFlow step.                 |
| `client.py`         | A python client of sqlflow_server.                           |
| `_database.so`      | Wrapper of the golang package `pkg/database`.                |
| `database.py`       | Wrapper to `_database.so` for ease of use.                   |
| `ir_pb2.py`         | Generated from `pkg/proto/ir.proto`.                         |
| `contracts.py`      | See below.                                                   |
| `diagnostics.py`    | See below.                                                   |

#### Python Version And The Style

The `sqlflow` package will only run in the *Docker container* and never be submitted to the PAI platform, so we propose to use python3 in the development.

We propose to follow the [Google Style Guide](http://google.github.io/styleguide/pyguide.html) for the `sqlflow` package. As you'll see, the example code in this document uses [type annotations](http://google.github.io/styleguide/pyguide.html#2214-decision) extensively. 

### Outline Design

In this part, we describe the outline design of several important modules. Note that this is not a substitute for detailed design, each topic below should have its own detailed design docs.

Implementing all of this at once will be a lot of work. To ensure the overall progress is controllable. The migration should be completed in two phases: 

1. The first phase or the **MVP** phase should implement an minimal viable product that duplicates the functionality of the current architecture.
2. The second phase should implement the new features such as the new `COLUMN` clause and `TO RUN`.

#### Prerequisites

In this part, we describe the outline design of modules that other modules depend on. The detailed design of these modules is the most important and should be done first.

##### The Database Module

As described above, we'll wrap the go `database` with `goalisa`, `gohive`, and `gomaxcompute` into a python module `_database.so`, we have to wrap the functionalities of `_database.so` into a thin wrapper `database.py` to convert golang objects into python objects.   `database.py`should expose at least the following functions/classes:

- `database.OpenDB(dbstr)`: parses `dbstr` and returns the appropriate `DB` class.
- `database.DB.Exec(self, stmt):` executes a query without returning any rows.
- `database.DB.Query`(self, stmt): executes a query that returns `Rows` , typically a SELECT
- `database.DB`: the object of a database connection
- `database.Rows`: the object returned by `database.DB.Query`, should be a python generator

##### Semantic Checking And Diagnostics

As described in [The Flaws In The Current Architecture](#the-flaws-in-a-nutshell), SQLFlow executes ML tasks in python, but users are always only familiar with SQL. To generate user friendly diagnostics, we must check whether the user inputs satisfy the requirements of the python functions or callables that will be eventually called.

The most widely-accepted solution to this problem is [contract programming](https://en.wikipedia.org/wiki/Design_by_contract). It enables defining formal, precise, and verifiable interface specifications for software components such as functions and callables. What we've done in the [sql/codegen/attribute](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/pkg/attribute) package is actually `contracts` in a limited way.

###### Python

We propose to design a contracts based mechanism in `contracts.py` for semantic checking and improving our daily development in python. The module should be able to:

1. Check inputs for existing models thoroughly, for example, `canned estimators` or `xgboost`.
2. Provide model developers an ease-to-use API to contribute models with thoroughly checking
   - `docstring` or `decorator` 
3. Throw an exception that incorporates all the violations to the contracts.
4. Require a custom model to provide thoroughly checking for each input parameters.
   - Making sure customized models will not reduce user experience

Not all runtime errors can be catched by the contracts based design, for example, training binary classifiers with dataset that has three classes. Therefore, besides `contracts.py`, we need another mechanism to catch such errors and pass a user-friendly diagnostics messages to the user. The mechanism would be implemented in `diagnosics.py`.

###### Golang

The golang package `executor` should parse the diagnostics messages generated by  `diagnostics.py` and return them to the users.

In the remainder of the section, we say **diagnostics are required** where `diagnostics.py` should be involved to generate an error message.

##### Intermediate Representations

As described above, the proposed architecture requires the golang part to pass the *ir* of a SQLFlow program to the python part to execute the program. We propose to define `ir` as a protobuf file in `pkg/proto/ir.proto`. It should have at least the following messages and fields, each other detailed design doc could add necessary fields to the `Statement` message.

```protobuf
message Statement {
  // `select` is the query for fetching data. For example,
  // "select * from iris.train;"
  optional string select = 2;
  // `validation_select` is the query for fetching the validation data.
  // For example, "select * from iris.val;".
  optional string validation_select = 3;
  // `model_image` is the name of the model's Docker image, for example, in the
  // statement "TO TRAIN a_data_scientist/regressors:v0.2/MyDNNRegressor", the
  // name "a_data_scientist/regressors:v0.2" is a Docker image.
  optional string model_image = 4;
  // `estimator` specifies the estimator type. For example, after parsing
  // "select ... train DNNClassifier WITH ...", `estimator` will be
  // "DNNClassifier".
  optional string estimator = 5;
  // `attributes` is a map of parsed attribute in the WITH Clause. For example,
  // after parsing "select ... to train ... with train.epoch=1000,
  // model.hidden_units = [10, 10]", the `attributes` will be
  // {"train.epoch": "1000", "model.hidden_units": ""[10, 10]""}
  optional map<string, string> attributes = 6;
  repeated string label = 7;
  message Columns {
  // The COLUMN clause will be split into `column`. For example, in the COLUMN
  // clause "COLUMN NUMERIC(sepal_length), INDICATOR(ID)" will be saved in
  // `columns` as ["NUMERIC(sepal_length)", "INDICATOR(ID)"]
    repeated string columns = 1;
  }
  // `columns` contain a map of string to `Columns` For multiple COLUMN clauses
  // like "COLUMN ... FOR deep_feature, COLUMN ... FOR wide_feature". They will
  // be parsed as {"deep_feature": [...], "wide_feature": [...]}. For single
  // column clause like "column ...", "feature_columns" will be used as the
  // default map key.
  optional map<string, Columns> columns = 8;
  // `model_save` specifies the saved model path in the INTO/USING clause.
  optional bool model_save = 9;
  enum Type {
    RUN = 0;
    TRAIN = 1;
    PREDICT = 2;
    EXPLAIN = 3;
    EVALUATE = 4;
    SOLVE = 5;
  }
  optional Type type = 10;
  // `predict_target` specifies the column to store predict result.
  // For example: "iris.predict.class"
  optional string predict_target = 11;
  // `original_sql` is the original statement from that this `Statement` is
  // generated. This can be for diagnostic purpose.
  optional string original_sql string = 100;
}

message Program {
  // `datasource` is the connection string to the database
  optional string datasource = 1;
  // `statements` is a list of `Statement` to be executed as a workflow
  repeated Statement statements = 2;
}
```

###### Golang

The golang package `ir` should fill the `Program` message according to the parsed result.

#### Workflow And Platform

###### Python

As decribed earlier, we would reimplement `workflow` in python. 

The module `execute.py` should define at least the following  functions:

```python
def execute(program:sqlflow_ir_pb2.Program) -> str:
    """
    `execute` executes the SQLFlow `program` and returns the job id
    """
    pass
  
def fetch(workflow_id:str):
    """
    `fetch` gets output of the workflow of `workflow_id` in an incremental way
    """
    pass
  
def step(statement:sqlflow_ir_pb2.Statement):
    """
    `step` dispatches the `statement` to the `sqlflow_submitter` package according to `statement.type` and the platform configured
    """
    pass
  
def main(mode:str):
    """
    `main` parses a `sqlflow_ir_pb2.Program` from stdin and call `execute` or `step` or `fetch` to execute the Program
    """
    pass
  
if __name__ == '__main__':
    main(sys.argv[1] if len(sys.argv) > 1 else "execute")
```

The python function `execute` executes the SQLFlow `program` and should do at least the following tasks:

- Do semantic checking for the `program` using `contracts.py`. **Diagnostics are required**.
- Call `features.py:generate_analysis_steps` to generate additional data transforming steps. 
- Call `couler` to generate an Argo `.yaml` including all `statement` steps and the additional steps and submit the workflow to Kubernetes.

The python function `fetch` gets output of an executing `Program` in a incremental way.

The python function `step` executes a SQLFlow `statement` as a workflow step. `step` should do at least the following tasks:

- From the `platform` package, **locate** the python module of the **configured** platform.
  - The configure mechanism should be described in detail in the detailed design doc (the current architecture use the environment variable `SQLFlow_submitter` to config the platform)
  - The locating mechanism should be described in detail in the detailed design doc.
-  Call one of `train`, `predict`, `explain` ,`run`, `solve`, `evaluate` of the python module of the platform according to `statement.type`

There should be at least three modules in the `platform` package:

- `pai.py`
- `alisa.py`
- `default.py`

The functions `train`, `predict`, `explain` ,`run`, `solve`, `evaluate` provided by each module in `platform` should forward to `sqlflow_sumitter/xgboost`, `sqlflow_submitter/tensorflow`, etc. according to `statement.type` and `statement.estimator`. **Diagnostics are required**.

The functions `train`, `predict`, `explain` ,`run`, `solve`, `evaluate `  provided by each module in `platform` should have the same signature. The signature should be defined in the detailed design doc.

###### Golang

To execute a SQLFlow program, the golang package `pkg/executor` should eventually spawn a python process that calls `execute.py:main("execute")`, and passes the filled `Program` to stdin of the process via the standard IPC pipe.

To run a step of a executing SQLFlow program, the golang package `pkg/step` should spawn a python process that calls `execute.py:main("step")`, and passes the filled `Program` (with only one `Statement`) to stdin of the process via the standard IPC pipe.

To get the status of an executing SQLFlow program, the golang package `pkg/server` should eventually spawn a python process that calls `execute.py:main("fetch")` .

###### MVP

The python function `fetch` is **not** required in the MVP.

#### Feature Derivation And Columns

###### Python

The function calls in the COLUMN clause should be defined as python functions in python modules in the `columns` package, for example, the `COLUMN numeric_column(x)` statement would directly map to the python function `numeric_column`. We should have at least a  `tf_columns.py`  module under this package to implement currently supported columns such as `numeric_column` or `indicator`.

`features.py` should have at least two function: 

```python
# Columns is the pseudo type of `map<string, Columns>` in the `Statement` message.
def generate_analysis_steps(datasource:str, columns:Columns):
    """
    `generate_analysis_steps` generates a `select` statement to get necessary aggregate data for feature columns
    """
    pass
  
# ColumnsDAG is a structure to be designed in the detailed design doc.
def generate_feature_columns(columns: ColumnsDAG):
    """
    `generate_feature_columns`
    """
    pass
```

`generate_analysis_steps` does at least the following tasks:

- Call the standard module  `ast`  to generate ASTs for the function call expressions in `columns`
- Do semantic checking for every function nodes in the ASTs. **Diagnostics are required.**
- Collect analysis requirements of the column functions from the nodes in the AST and construct analysis `select` statements
- Construct a `ColumnsDAG` that represents the `columns` from the AST
- Return the DAG and the analysis `select` statements to `workflow`. `workflow` should put the DAG in an Argo artifact that could be passed to the following steps. 

`generate_feature_columns` does at least the following task:

- Generate python column objects ( `tf.feature_columns` or a function that returns tensors e.g.) according to the `ColumnsDAG` and the results of the analysis `select` statements (which could be in a database table or in an artifact)

`ColumnsDAG` and the columns to be implemented should be defined in the detailed design doc.

###### Golang

The  `parser` package should verify the syntax of the `COLUMN` clause from user inputs, The `ir` package should fill the `columns` field of the `Statement` message.

###### MVP

`generate_analysis_steps` is **not required in the** **MVP**.

#### The Python Client

There should be at least one function in `client.py`:

```python
def sqlflow(stmt:str, server_addr:str, datasource:str):
    '''
    `sqlflow` takes a SQLFlow statement and submits it to the sqlflow_server listening server_addr
    '''
    pass
```

The `sqlflow` function simply connects to sqlflow_server running at the `server_addr`, the server would then create a `Program` for this single statement with `datasource` and start a python process to `execute` the program.

## Does the Proposed Architecture Solve the Flaws in the Current Architecture?

To answer the question, let's review the scenarios in the beginning of the document.

### Scenario 1: An ML Expert Wants A New Column Type 

Adding `image_embedding_column `  in the proposed architecture requires the following main steps:

1. Implement the column in a python function in a python file under the `columns` package. 
2. Test and debug: a bug can only be in the python function.

### Scenario 2: An ML Expert Wants To Contribute A New Type Of Model 

Adding `LightGBM` to SQLFlow, this requires the following main steps:

1. Add a directory `lightgbm` in [python/sqlflow_submitter](https://github.com/sql-machine-learning/sqlflow/tree/400c691470c6503393453d47856913df3365503e/python/sqlflow_submitter). 
2. Write `train.py`, `predict.py`, `explain.py`, `evaluate.py`  in the `lightgbm` directory, each file has a function with the same name as the file, and the functions implements the main logic for SQLFlow train statement, predict statement, explain statement, and evaluate statement, respectively.
3. Test and debug: a bug can be only in the python files.

The answer is obvious.

## To Be Prepared

To make sure the existing code is still working before we finish the migration. We have to define a command-line flag `--python_refactory` and use that flag to isolate the code of the proposed architecture.

- A tiny modification can be implemented as:

  ```go
   # In some function
      if !flags.python_refactory {
          // The existing logic
      } else {
          // The new logic
      }
  ```

  - After the refactory, we remove the flag and all the legacy code

- Big changes can be written as:

  ```go
  func generateTrainStmt // ...
  func generateTrainStmtForPython // ...
  
  struct SQLStatement {}
  struct SQLStatementForPython {}
  ```

  or even

  ```
  pkg/ir/
  pkg/ir2/
  ```

  if the change is big enough.

- After the migration, remove the flag and all the legacy code and rename the `pkg`s, `func` s and `struct`s by removing the suffix `ForPython`.
