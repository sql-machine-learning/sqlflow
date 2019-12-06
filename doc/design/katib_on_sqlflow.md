# Katib on SQLFlow
This design is about optimizing model hyperparameters in SQLFlow by creating Katib step via couler.

## Requirements:

Currently, SQLFlow allows programmers to use SQL queries to enable ML model training, prediction, and explanation. However, hyperparameters optimization is also a necessary function that can help users to tune hyperparamters of their models in an easy way. This function will be supported by Katib in SQLFlow.

## Hyperparameter Optimization in Katib

[Katib](https://github.com/kubeflow/katib) is a Kubernetes Native System for Hyperparameter Tuning and Neural Architecture Search. The system is inspired by Google Vizier and supports multiple ML/DL frameworks (e.g. TensorFlow, Apache MXNet, PyTorch and XGBoost).

A training job in Katib is called an experiment. Users need to specify or choose: 
- parameters need to be optimized in the training. 
- the value range for each parameter.
- the algorithm (e.g., `random`) used to choose candidate value for training paramters.
- objective (e.g., accuracy) which measures the model with different parameters' value.

When an experiment is done, users can check objectives of each set of training parameters' value via web UI. Then users can decide the best value for those training parameters.

## Couler and SQLFlow

SQLFlow does not create training jobs on Katib directly. Instead, SQLFlow creates Katib steps in a workflow via couler, and the workflow will be run by Argo on Kubernetes. More details can be found in [couler and SQLFlow](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/couler_sqlflow.md). 

## SQLFlow Syntax

In order to use Katib, programmers need to specify it is a Katib job by `TRAIN katib.{tf, pytorch, xgboost}.{...}` in SQL query. Here we use a simple example to explain the syntax of creating Katib job for XGBoost training in SQLFlow.  

``` sql
SELECT * FROM train_table
TO TRAIN a_data_scientist/xgboost:v0.5/katib.xgboost.gbtree
WITH
    objective=multi:softmax,
    eta=1,
    num_round=[20, 100],
    max_depth=[],
    validatation_dataset="select * from test_table"
LABEL class
INTO my_xgb_model;
```
The above example,
- This query tries to train a XGBoost model in Katib. 
- `a_data_scientist/xgboost:v0.5/katib.xgboost.gbtree`
    - `a_data_scientist/xgboost:v0.5`: container image source
    - `katib.xgboost.gtree`:
        - `katib` indicates to create a Katib step via couler.
        - `xgboost` indicates to train a XGBoost model.
        - `gbtree` indicates the booster type in XGBoost model training. 
- In the `WITH` clause:
    - `objective` and `eta` are parameters in XGBoost. More details about XGBoost parameters see: [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters). Those parameters are optional. If users do not specify them, those parameters will be filled by default value. 
    - `num_round` and `max_depth` are parameters to be optimized. If programmers specify the value range for a parameter (e.g., `num_round`), the range will be set for this parameter during training; otherwise, programmers assign an empty list to a parameter (e.g., `max_depth`), this parameter will be assigned default range during training. If programmers do not specify any parameters to be optimized, the system will optimize default parameters for this model.   
    - `validation_dataset` is required in Katib jobs. This indicates the data used for testing in model training in Katib.  
- `my_xgb_model` is the name of the trained model. 


## Required update in SQLFlow Couler codegen:

In SQLFlow, Couler codegen will generate a python program from the input SQL statement. In the generated python program, it invokes Couler APIs (`couler.run_container(...)`) to create Argo steps. However, the current generated python program only calls `couler.run_container` to create a single container step, which does not work with job type steps (e.g., Katib). The container step runs a single container while a job step may launch multiple containers which will coordinate with each other. 

Then it needs to invoke different Couler APIs according to input SQL queries. In Katib case, Couler codegen needs to check if the input SQL query is to create a Katib job. If it is, the codegen should generate codes to invoke a Couler Katib API, like `couler.run_katib(...)`

## Design for `couler.run_katib(...)`

Considering Katib itself supports multiple models and frameworks, and more may come in the future, we use a unique API to create Katib steps in Couler. The API is as following:

`couler.run_katib("model_or_framework"=None, katib_params={}, model_or_framework_params={})`

In this API, it includes three arguments:
- `model_or_framework`: string, indicates the model (e.g., XGBoost) or frameworks (e.g., tf or pytorch) used in model training on Katib.
- `katib_params`: dict, configures Katib jobs. For example, users can specify max trials in model training, like `katib_params= { "max_trial_count": 10}`  
- `model_or_framework_params`: dict, configures model or framework given in `model_or_framework`. The above XGBoost example will be: `model_or_framework_params= { "booster": "gbtree", "objective": "multi:softmax", "eta": 1, "num_round": [20, 100], max_depth: [] }`
