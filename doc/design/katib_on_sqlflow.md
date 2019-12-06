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

For boosting tree models, especially models with XGBoost, there is a small group of effective hyperparameter, and we can empirically determine their ranges.  We noticed that the following two are the most important.

- `max_depth` in the range [2,10], and
- `num_round` in the range [50, 100].

With the introduction of auto hyperparameter tuning, we hope that users don't need to specify the `num_round` and `max_depth` values in the following SQL statement.

```sql
SELECT * FROM a_dataset_table
TO TRAIN a_data_scientist/xgboost_models:v0.5/a_gbtree_model
WITH
    objective=multi:softmax,
    eta=1,
    num_round=[20, 100],
    max_depth=[],
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

SQLFlow parses each SQL program into an IR, which is a list of statement IRs.  The `codegen_couler.go` converts the IR into a Couler program.   We need to add a Couler functions `couler.katib.train` for the calling by the generated Couler program.

Consider the following example program.

```sql
SELECT * FROM a, b WHERE a.id = b.id INTO c;
SELECT * FROM c TO TRAIN model_def WITH objective=multi:softmax, eta=1 LABEL class INTO my_xgb_model;
```

The `codegen_couler.go` might generate the following Couler program.

```python
couler.maxcompute.run("""SELECT * FROM a, b WHERE a.id = b.id INTO c;
                         SELECT * FROM c INTO temp""")
couler.maxcompute.export(table="temp", file="/hdfs/temp")
couler.katib.train(model_def, data="/hdfs/temp")
```

## `couler.katib.train(...)`

Considering Katib itself supports multiple models and frameworks, and more may come in the future, we introduce the following Couler function.

```python
def couler.katib.train(model_def=None, hyperparameters={})
```
