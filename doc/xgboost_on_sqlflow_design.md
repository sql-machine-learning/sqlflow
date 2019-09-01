# Design Doc: XGBoost on SQLFlow

## Introduction

This design doc introduces how  users can train/predict the [XGBoost](https://xgboost.ai/) model by SQLFlow SQL and how
we implement it.

## Design

We prefer users to execute the SQLFlow Train/Predict SQL as follows:

  ``` sql
  SELECT * FROM train_table
  TRAIN XGBoost
  WITH
      train.objective="multi:softmax",
      train.num_round=2,
      model.max_depth=2,
      model.eta=1
  LABEL class
  INTO my_xgb_model;
  ```
  
  ``` sql
  SELECT * FROM test_table
  PREDICT pred_table.result
  USING my_xgb_model;
  ```

where:
- `my_xgb_model` is the trained model.
- `XGBoost` means to train an XGBoost model instead of the TensorFlow Model.
- The prefix `train.` in `WITH` statement mappings to the training arguments of XGBoost [train function](https://xgboost.readthedocs.io/en/latest/python/python_api.html#xgboost.train).
- The prefix `model.` in `WITH` statement mappings to the [XGBoost Parameters](https://xgboost.readthedocs.io/en/latest/parameter.html);

`codegen_xgboost.go` would generate an XGBoost Python program including:
- Generate the XGBoost input database.
- Pass the train/predict parameters to XGBoost Python program.
- Save the trained model.

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
