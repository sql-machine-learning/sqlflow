# XGBoost on SQLFlow

## Introduction

This design explains how SQLFlow calls [XGBoost](https://xgboost.ai/) for training models and prediction.

## Usage

To explain the benefit of integrating XGBoost with SQLFlow, let us start with an example.  The following SQLFlow code snippet shows how users can train an XGBoost tree model named `my_xgb_model`.

``` sql
SELECT * FROM train_table
TO TRAIN xgboost.gbtree
WITH
    objective=multi:softmax,
    train.num_boost_round=2,
    max_depth=2,
    eta=1
LABEL class
INTO my_xgb_model;
```

The following example shows how to predict using the model `my_xgb_model`.

``` sql
SELECT * FROM test_table
TO PREDICT pred_table.result
USING my_xgb_model;
```

The the above examples,
- `my_xgb_model` names the trained model.
- `xgboost.gbtree` is the model name, to use a different model provided by XGBoost, use `xgboost.gblinear` or `xgboost.dart`, see: [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters) for details.
- In the `WITH` clause,
  - objective names an [XGBoost learning task](https://xgboost.readthedocs.io/en/latest/parameter.html#learning-task-parameters)
  - keys with the prefix `train.` identifies parameters of XGBoost API [`xgboost.train`](https://xgboost.readthedocs.io/en/latest/python/python_api.html#xgboost.train), and
  - keys without any prefix identifies [XGBoost Parameters](https://xgboost.readthedocs.io/en/latest/parameter.html) except the `objective` parameter, which was specified by the identifier after the keyword `TO TRAIN`, as explained above.

## The Code Generator

The code generator `codegen_xgboost.go` outputs an XGBoost program in Python. It contains the following features:
1. It tells the SQL engine to run the SELECT statement and retrieve the training/test data. It saves the data into a text file, which could be loaded by XGBoost using the DMatrix interface.
1. Parse and resolve the WITH clause to fill the `xgboost.train` arguments and the XGBoost Parameters.
1. Save the trained model on disk.
1. For the TO PREDICT clause, it loads the trained model and test data and then outputs the prediction result to a SQL engine.
