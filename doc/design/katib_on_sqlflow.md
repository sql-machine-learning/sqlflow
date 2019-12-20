# Auto Hyperparameter Tuning

SQLFlow allows the users to specify hyperparameter values via the `WITH` clause when training models.  However, most users under our survey prefer that SQLFlow could automatically estimate these hyperparameters instead.  This document is about the automatic hyperparameter estimation.

## Katib

[Katib](https://github.com/kubeflow/katib) is a Kubernetes Native System for Hyperparameter Tuning and Neural Architecture Search.  The inspiration of Katib comes from Google Vizier and supports multiple machine learning frameworks, for example, TensorFlow, Apache MXNet, PyTorch, and XGBoost.  We compared Katib with some other auto hyperparameter tuning systems, and we prefer its Kubernetes-native architecture.

However, Katib, or hyperparameter tuning in the academic literature, is not sufficient for our use case.

## The Paradox

To define a training job, a.k.a., *experiment*, in Katib, users need to specify the search range of each hyperparameter.

Ironically, it is an extra burden for the users to specify the above information, as our goal is to release users from specifying hyperparameters.

## Untangle the Paradox

### Boosting Tree Models

For boosting tree models, especially models with XGBoost, there is a small group of effective hyperparameters, and we can empirically determine their ranges.  We noticed that the following two are the most important.

- `max_depth` in the range [2,10], and
- `num_round` in the range [50, 100].

With the introduction of auto hyperparameter tuning, we hope that users don't need to specify the `num_round` and `max_depth` values in the following SQL statement.

```sql
SELECT * FROM train_data_table
TO TRAIN a_data_scientist/xgbooost:v2/xgboost.gbtree
WITH
    objective=multi:softmax,
    eta=0.1,
    range.num_round=[50, 100],
    range.max_depth=[2, 8],
    validation_dataset="SELECT * FROM test_data_table;"
LABEL class
INTO my_xgb_model;
```

### Deep Learning Models

For deep learning models, the case is complicated.  Each model has its own set of hyperparameters, and their ranges might vary significantly.  Our proposed solution is to utilize the [model zoo](model_zoo.md).  In particular, users might train a model defined in the zoo with various datasets, in various experiments, with manually specified hyperparameters.  After the training, some users might publish their trained models, including the estimated parameters and the specified hyperparameters.  Given these published hyperparameter values, the [Bayesian hyperparameter optimization](https://en.wikipedia.org/wiki/Hyperparameter_optimization#Bayesian_optimization)  for hyperparameter tuning.  We are working on such a Bayesian approach that doesn't require explicit specification of hyperparameter ranges.  We plan to contribute it to Katib.

### Trigger Hyperparameter Tuning

Each model definition has a specification listing its hyperparameters.  If the user-specified values of all of them, there is no need for tuning; otherwise, SQLFlow should call Katib.

## The System Design

### From Submitter to Couler

SQLFlow has been working as converting a SQL program into a Python program known as a *submitter* before executing the submitter.  However, we recently realized that the idea of the submitter is insufficient for cloud deployment.  As Kubernetes might preempt the SQLFlow server, it could lose the status of the execution of submitters.

This observation urges us to make the following changes.

1. Introducing a workflow engine, namely [Argo](http://argoproj.io/).
1. Make SQLFlow generates a workflow instead of a Python program.
1. SQLFlow server submits the workflow to Argo for the execution.
1. Argo manages the status of workflow executions.

Argo takes workflows in the form of YAML files, and it is error-prone to write such YAML files manually.  So, we created [Couler](/python/couler/README.md) as an intermediate programmatic representation of workflows.

We need to develop a new codegen, `codegen_couler.go`, for SQLFlow.  `codegen_couler.go` converts the parsed SQL program, a.k.a., the [intermediate representation](/pkg/sql/ir), or IR, into a Couler program.

### The Integration via Couler

SQLFlow parses each SQL program into an IR, which is a list of statement IRs.  The `codegen_couler.go` converts the IR into a Couler program.   We need to add a Couler functions `couler.sqlflow.train` for the calling by the generated Couler program.

Consider the following example program.

```sql
SELECT * FROM a, b WHERE a.id = b.id INTO c;
SELECT * FROM c TO TRAIN data_scientist/xgboost:v0.5/xgboost.gbtree 
    WITH objective=multi:softmax, eta=0.1, validation_dataset="select * from d;" 
    INTO my_xgb_model;
```

The `codegen_couler.go` might generate the following Couler program.

```python
couler.maxcompute.run("""SELECT * FROM a, b WHERE a.id = b.id INTO c;""")
couler.sqlflow.train(model="xgboost.gbtree", hyperparameters={"objective": "multi:softmax", 
    "eta": 0.1},  image="data_scientist/xgboost:v0.5",
    sql="select * from c to train data_scientist/xgboost:v0.5/xgboost.gbtree ... ",
    datasource="mysql://..." )
```

## `couler.sqlflow.train(...)`

Considering Katib itself supports multiple models and frameworks, and more may come in the future, we introduce the following Couler function.

```python
def couler.sqlflow.train(model=None, hyperparameters={}, image=None, sql=None,datasource=None)
```

The arguments in `couler.sqlflow.train`,

- `model` defines the training model, e.g., `xgboost`.
- `hyperparameters` specifies hyperparameters for model given in `model`.
- `image` specifies the container image source for the Katib tuning job.
- `sql` sql statement input by users.
- `datasource` train and validation data source 

## Run Tuning Job on Katib

In each Katib tuning job, users need to define tuning parameters (i.e., the hyperparameter's name, type, and range) in a model at first. During runtime, the Katib will pick up different values for those hyperparameters and start a single Pod for each value set. Then the tuning job Pods, which are running customized container image, must follow the Katib input format and take those hyperparameters' values from Katib, to train and measure the model.

For example, users may define the following command for tuning job Pod:

`python -m sqlflow_submitter.couler.katib.xgboost_train`

The actual command during runtime will be:

`python -m sqlflow_submitter.couler.katib.xgboost_train --max_depth 5 ...`, hyperparameter `max_depth` is added by Katib.

We have proposed two different designs here. Assuming users have input the following SQL statement:

```sql
SELECT * FROM a, b WHERE a.id = b.id INTO c;
SELECT * FROM c TO TRAIN data_scientist/xgboost:v0.5/xgboost.gbtree 
    WITH objective=multi:softmax, eta=0.1, range.max_depth=[2, 10],validation_dataset="select * from d;" 
    INTO my_xgb_model;
```

### Pass all parameters to the tuning Pod command

In this design, SQLFlow parses the input SQL statement and extract all parameters, for example, model parameters, train and validation data select, etc. Then SQLFlow transforms all those parameters to strings and passes them to `couler.sqlflow.train`. 

In the SQLFlow submitter, it invokes `couler.sqlflow.train`:

`couler.sqlflow.train(model="xgboost", hyperparameters={"objective": ... }, params={"select": ..., "features": ...}, datasource=... )`

Then in tuning Pod, it executes the following command:

`python -m xgb_train --select "select ..." --features "{...}" ... --max_depth 5 ...`

### Pass SQL to tuning Pod and extract parameters in the Pods

In this design, SQLFlow parses the input SQL statement but only extract tuning hyperparameters. Then SQLFlow passes the whole SQL statement to `couler.sqlflow.train`. 

In the SQLFlow submitter, it invokes `couler.sqlflow.train`:

`couler.sqlflow.train(model="xgboost", hyperparameters={"objective": ... }, sql="select ...", datasource= ... )`

Then in tuning Pod, it executes following command:

`python -m xgb_train --max_depth 5`

In xgb_train.py, it runs following codes:

```python
setenv("MAX_DEPTH", 5)
run_cmd("repl -m \"select * ...\" ")

```

## Pipeline

The pipeline from SQL statements to Argo workflow:

- SQLFlow generates `IR` from input SQL statements.
- `couler_katib_codegen.go` takes this `IR` as input and obtains parameters for Katib training job.
- `couler_katib_codegen.go` generates a Python program which invokes `couler.sqlflow.train`. At the same time, `couler_katib_codegen.go` fills this API's arguments with Katib parameters.
- `couler.sqlflow.train` generates the manifest for Katib job and fills it in Argo workflow yaml as a step.
- To execute Argo workflow on Kubernetes and Argo runs Katib job.
