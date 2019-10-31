# SQLFlow Trains Models via XGBoost in Katib

## Requirement description:

SQLFlow needs to provide users with a simple way to train their ML model. This function is supported by SQLFlow running XGBoost jobs on Katib.

## Hyperparameter Optimization in Katib

[Katib](https://github.com/kubeflow/katib) is a Kubernetes Native System for Hyperparameter Tuning and Neural Architecture Search. The system is inspired by Google vizier and supports multiple ML/DL frameworks (e.g. TensorFlow, MXNet, and PyTorch).

## Support XGBoost job in Katib

[XGBoost](https://xgboost.readthedocs.io/en/latest/) is an optimized distributed gradient boosting library designed to be highly efficient, flexible and portable. 

When to run XGBoost jobs in Katib, we create an Experiment CR on Kubernetes via a yaml file. This Experiment CR will create one Suggestion CR and multiple Trail Pods later. The Suggestion CR generates the value for hyperparameters. Each Trail Pod include two containers: a job container and a MetricsCollector container. The job container is created from a standard XGBoost Docker image created by us. When the job container is started, it receives paramters defined in SQLFlow as well as value of hyperparameters from Suggestion CR.

MetricsCollector container parse logs of the job containers and put training results into Katib data store. When the training job is complete, SQLFlow can read results from Katib data store.

## Pipeline:

1. Users input SQL queries. 
2. Based on input SQL queries, the codegen `codegen_katib_xgboost.go` generates `katib_xgboost.py` file. This file include all parameters for XGBoost jobs. 
3. SQLFlow server executes `katib_xgboost.py`:
   1. generates `katib_xgboost.yaml` file and fill it with: 
      1. the scope of hyperparameters
      2. source of standard XGBoost Docker image
      3. commands to execute `train_xgboost.py` in the container
   2. executes `katib_xgboost.yaml` on kubernetes and start XGBoost jobs in Katib.

   

