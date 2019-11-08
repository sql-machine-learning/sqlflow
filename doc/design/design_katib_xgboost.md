# SQLFlow Optimize Model Hyperparameters via XGBoost in Katib

## Requirements:

SQLFlow allows programmers to use SQL queries to enable ML model training, prediction and explaination. SQLFlow also allows programmers to tune hyperparamters of their models in an easy way. This function (Hyperparameter Optimization) is supported by running HPO jobs on Katib.

To provide better support to programmers, SQLFlow allows programmers to specify a particular framework or model to tune their hyperparameters. XGBoost is among the most popular models. 

## SQLFlow Syntax

Here we use a simple example to explain the syntax of SQLFlow.  

``` sql
SELECT * FROM train_table
TRAIN xgboost.gbtree
WITH
    objective=multi:softmax,
    eta=1
LABEL class
INTO my_xgb_model;
```
The the above examples,
- This query tries to tune XGBoost model. 
- `xgboost.gtree` configures the booster used. 
- In the `WITH` clause:
    - `objective`indicates objective parameter in XGBoost. 
    - `eta` indicates eta parameter in XGBoost, more details see: [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters) for more details of XGBoost parameters.
- `my_xgb_model` file includes optimized hyperparameter value.
  

## Hyperparameter Optimization in Katib

[Katib](https://github.com/kubeflow/katib) is a Kubernetes Native System for Hyperparameter Tuning and Neural Architecture Search. The system is inspired by Google Vizier and supports multiple ML/DL frameworks (e.g. TensorFlow, Apache MXNet, and PyTorch).

## Support XGBoost job in Katib

[XGBoost](https://xgboost.readthedocs.io/en/latest/) is an optimized distributed gradient boosting library designed to be highly efficient, flexible and portable. 

When to run XGBoost jobs in Katib, we create an Experiment CR on Kubernetes via a yaml file. This Experiment CR will create one Suggestion CR and multiple Trial Pods later. The Suggestion CR generates the value for hyperparameters. Each Trail Pod includes two containers: a job container and a MetricsCollector container. The job container is created from a standard XGBoost Docker image created by us. When the job container is started, it receives parameters defined in SQL query and the value of hyperparameters from Suggestion CR.

MetricsCollector container parses logs of the job containers and put training results into Katib data store. When the training job is complete, SQLFlow can read results from Katib data store.

## Pipeline:

1. Users input SQL queries. 
2. Based on input SQL queries, the codegen `codegen.go` generates `katib_xgboost.py` file. This file includes all parameters for XGBoost hyperparameters optimization.
3. SQLFlow server executes `katib_xgboost.py`:
   1. read experiment specifics from `template_katib_xgboost.yaml` file.
   2. fill paramters for XGBoost experiment:
      1. experiment name.
      2. the scope of hyperparameters: now the scope of hyperparameters are constant value. Later there will be a `model_zoo` to generate optimized scope for each hyperparameters.
      3. XGBoost model parameters.
   3. submit XGBoost experiment to Katib.

   

