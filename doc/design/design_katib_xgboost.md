# Katib XGboost on SQLFlow

## Overview:
This is a doc on integration with katib XGboost.

## User Interface:

We use a couple of examples to explain the usage of katib xgboostith.  The following SQLFlow code snippet shows how users can train an XGBoost tree model named `my_xgb_model`.

``` sql
SELECT * FROM train_table
TRAIN xgboost.gbtree
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
PREDICT pred_table.result
USING my_xgb_model;
```

The the above examples,
- `my_xgb_model` names the trained model.
- `xgboost.gbtree` is the model name, to use a different model provided by XGBoost, use `xgboost.gblinear` or `xgboost.dart`, see: [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters) for details.
- In the `WITH` clause,
  - objective names an [XGBoost learning task](https://xgboost.readthedocs.io/en/latest/parameter.html#learning-task-parameters)
  - keys with the prefix `train.` identifies parameters of XGBoost API [`xgboost.train`](https://xgboost.readthedocs.io/en/latest/python/python_api.html#xgboost.train), and
  - keys without any prefix identifies [XGBoost Parameters](https://xgboost.readthedocs.io/en/latest/parameter.html) except the `objective` parameter, which was specified by the identifier after the keyword `TRAIN`, as explained above.

## Implementation:

Steps:
1. Based on input SQL statement, the codegen generates a katib-xgboost.py file and is submitted to SQLflow server.
2. SQLflow server executes katib-xgboost.py:
   1. to generate train_xgboost.py file. All input parameters are filled in train_xgboost.py.
   2. to generate a Dockerfile and build a docker image based on this Dockerfile. The train_xgboost.py and required data file will be copied into this image at the same time.
   3. to push this docker image into docker.io repository.
   4. to generate a katib-xgboost.yaml and fill it with: (1) source of docker image generated above; and (2) commands to execute train_xgboost in the container.
   5. to submit and execute katib-xgboost.yaml on kubernetes.
3. Katib creates an experiments to run XGboost training job.

