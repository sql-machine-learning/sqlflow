# Design Doc: XGBoost on SQLFlow

## Introduction

This design explains how SQLFlow calls [XGBoost](https://xgboost.ai/) for training models and prediciton.

## Usage

To explain the benefit of integrating XGBoost with SQLFlow, let us start with an example.  The following SQLFlow code snippet shows how users can train an XGBoost tree model named `my_xgb_model`.

``` sql
SELECT * FROM train_table
TRAIN xgboost.multi.softmax
WITH
    train.num_round=2,
    max_depth=2,
    eta=1
LABEL class
INTO my_xgb_model;
```

The following example shows how to predict using the model `my_xgb_model`.

``` sql
SELECT * FROM test_table
PREDICT pred_table.result
USING my_xgb_model;
```

The the above examples,
- `my_xgb_model` names the trained model.
- `xgboost.multi.softmax` is the model spec, where
    - the prefix `xgboost.` tells the model is a XGBoost one, but not a Tensorflow model, and
    - `multi.softmax` names an [XGBoost learning task](https://xgboost.readthedocs.io/en/latest/parameter.html#learning-task-parameters).
- In the `WITH` clause, 
  - keys with the prefix `train.` identifies parameters of XGBoost API [`xgboost.train`](https://xgboost.readthedocs.io/en/latest/python/python_api.html#xgboost.train), and
  - keys without any prefix identifies [XGBoost Parameters](https://xgboost.readthedocs.io/en/latest/parameter.html) except the `objective` parameter, which was specified by the identifier after the keyword `TRAIN`, as explained above.

## The Code Generator

The code generator `codegen_xgboost.go` outputs an XGBoost program in Python. It contains the following features:
1. Transport the user-typed **SELECT STATEMENT** into [XGBoost DMatrix](https://xgboost.readthedocs.io/en/latest/python/python_api.html?highlight=dmatrix#xgboost.DMatrix) which is the Data Matrix used in XGBoost.
1. Fill the training control arguments and xgboost parameters according to the user-typed **WITH STATEMENT**.
1. Save the trained model on disk.
1. For the **PREDICT STATEMENT**, the submitter Python program would load the trained model and `dtest` which is a DMatrix object generated from **PREDICT SELECT STATEMENT** to output the prediction result.
