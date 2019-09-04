# Design Doc: XGBoost on SQLFlow

## Introduction

This design explains how SQLFlow calls [XGBoost](https://xgboost.ai/) for training models and prediciton.

## Usage

To explain the benefit of integrating XGBoost with SQLFlow, let us start with an example.  The following SQLFlow code snippet shows how users can train an XGBoost tree model named `my_xgb_model`.

``` sql
SELECT * FROM train_table
TRAIN xgboost.multi.softmax
WITH
    train.objective="multi:softmax",
    train.num_round=2,
    params.max_depth=2,
    params.eta=1
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
  - the prefix `params.` identifies [XGBoost Parameters](https://xgboost.readthedocs.io/en/latest/parameter.html) except the `objective` parameter, which was specified by the identifier after the keyword `TRAIN`, as explained above.

## The Code Generator

The code generator `codegen_xgboost.go` outputs an XGBoost program in Python. It contains the following features:
1. Generate the XGBoost input database.
1. Pass the train/predict parameters to XGBoost Python program.
1. Save the trained model.
1. Using [Learning API](https://xgboost.readthedocs.io/en/latest/python/python_api.html#module-xgboost.training) instead of [Sckiet-Learn API](https://xgboost.readthedocs.io/en/latest/python/python_api.html#module-xgboost.sklearn) just because we prefer explain the XGBoost model by [SHAP](https://github.com/slundberg/shap).

### Input Format

SQLFlow implements [db_generator](/sql/python/sqlflow_submitter/db.py#db_generator) that takes the 
`SELECT STATEMENT` as the input and outputs a iterable function which 
yields `(features, label)` for each iteration call. `codegen_xgboost` would reuse the `db_generator`
to generate the XGBoost input database.

XGBoost using `DMatrix` as the input structure, according to [Text Input Format of DMatrix](https://xgboost.readthedocs.io/en/latest/tutorials/input_format.html), we prefer to implement `XGBoostDatabase` that
takes `db_generator` as the input and outputs text files with LibSVM format.

- For the **basic** input format

    the train table can be like:

    ``` text
    col0 | col1 | col2 | label
    1.1 NULL 2.2 1
    0.8 2.0 2.2 2
    0.2 3.0 NULL 0
    0.77 4.0 2.6 2
    ```

    `codegen_xgboost.go` would write down the `train.txt` file like:

    ``` text
    1 0:1.1 2:2.2
    2 0:0.8 1:2.0 3:2.2
    0 0:0.2 1:3.0
    2 0:0.77 1:4.0 2:2.6 
    ```

- For the **group** input format, users can easy to specify the group column by `train.group_column` in the WITH statement like:

    ``` sql
    SELECT * FROM train_table
    TRAIN XGBoost
    LABEL class
    WITH
        train.group_column=group
    ...
    ```

    The group column in table can be like:

    ``` text
    col1 | col2| col3 | label | group
    1.1 2.0 2.2 1 1
    0.8 2.0 2.2 2 1
    0.2 3.0 4.2 0 2
    0.77 4.0 2.6 2 3
    ```

    `codegen_xgboost.go` would write down the `train.txt.group` file like:

    ``` text
    2
    1
    1
    ```

- For the **weight** input format, users can specify the weight column like `group`:

    ``` sql
    SELECT * FROM train_table
    TRAIN XGBoost
    LABEL class
    WITH
        train.weight_column=weight
    ```

    `codegen_xgboost.go` would also write the `train.txt.weight` file on the disk.
  
## TBD

- Implement auto-train feature to search the parameter.
- Support the sparse data format.
