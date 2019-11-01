# SQLFlow Optimize Model Hyperparameters via XGBoost in Katib

## Requirements:

SQLFlow allows programmers to use SQL queries to enable ML model training, prediction and explaination. SQLFlow also allows programmers to tune hyperparamters of their models in an easy way. This function (Hyperparameter Optimization) is supported by running HPO jobs on Katib.

To provide better support to programmers, SQLFlow allows programmers to specify a particular framework or model to tune their hyperparameters. XGBoost is among the most popular models. 

## SQLFlow Syntax

Here we use a simple example to explain the syntax of SQLFlow.  

``` sql
SELECT * FROM train_table, test_table
TUNE num_boost_round, max_depth
WITH
    framework=katib.xgboost
    param.model=xgboost.gbtree
    param.objective=multi:softmax,
    param.eta=1
INTO my_model_hp;
```
The the above examples,
- This query tries to tune hyperparameter `num_boost_round`, `max_depth`. 
- In the `WITH` clause:
    - `katib.xgboost`indicates to tune those hyperparameters by running XGBoost jobs on Katib. 
    - `xgboost.gbtree`, `objective` and `eta` are parameters for XGBoost model, see: [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters) for more details of XGBoost parameters.
- `my_model_hp` file includes optimized value for `num_boost_round` and `max_depth`.
  

## Hyperparameter Optimization in Katib

[Katib](https://github.com/kubeflow/katib) is a Kubernetes Native System for Hyperparameter Tuning and Neural Architecture Search. The system is inspired by Google Vizier and supports multiple ML/DL frameworks (e.g. TensorFlow, Apache MXNet, and PyTorch).

## Support XGBoost job in Katib

[XGBoost](https://xgboost.readthedocs.io/en/latest/) is an optimized distributed gradient boosting library designed to be highly efficient, flexible and portable. 

When to run XGBoost jobs in Katib, we create an Experiment CR on Kubernetes via a yaml file. This Experiment CR will create one Suggestion CR and multiple Trial Pods later. The Suggestion CR generates the value for hyperparameters. Each Trail Pod includes two containers: a job container and a MetricsCollector container. The job container is created from a standard XGBoost Docker image created by us. When the job container is started, it receives parameters defined in SQL query and the value of hyperparameters from Suggestion CR.

MetricsCollector container parses logs of the job containers and put training results into Katib data store. When the training job is complete, SQLFlow can read results from Katib data store.

## Pipeline:

1. Users input SQL queries. 
2. Based on input SQL queries, the codegen `codegen_katib_xgboost.go` generates `katib_xgboost.py` file. This file includes all parameters for XGBoost jobs. 
3. SQLFlow server executes `katib_xgboost.py`:
   1. generates `katib_xgboost.yaml` file and fill it with: 
      1. the scope of hyperparameters: now the scope of hyperparameters are constant value. Later there will be a `model_zoo` to generate optimized scope for each hyperparameters.
      2. source of standard XGBoost Docker image.
      3. commands to execute XGBoost job python program in the container.
   2. executes `katib_xgboost.yaml` on kubernetes and start XGBoost jobs in Katib.

   

