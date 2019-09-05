# _user guide:_ Ant-XGBoost on sqlflow

## Overview

[Ant-XGBoost](https://github.com/alipay/ant-xgboost) is fork of [dmlc/xgboost](https://github.com/dmlc/xgboost), which is maintained by active contributors of dmlc/xgboost in Alipay Inc.

Ant-XGBoost extends `dmlc/xgboost` with the capability of running on Kubernetes and automatic hyper-parameter estimation. 
In particular, Ant-XGBoost includes `auto_train` methods for automatic training and introduces an additional parameter `convergence_criteria` for generalized early stopping strategy.
See supplementary section for more details about automatic training and generalized early stopping strategy.

## Tutorial
We provide an [interactive tutorial](../example/jupyter/tutorial_antxgb.ipynb) via jupyter notebook, which can be run out-of-the-box in [sqlflow playground](https://play.sqlflow.org).
If you want to run it locally, you need to install SQLFlow first. You can learn how to install sqlflow at [here](../doc/installation.md).

## Concepts
### Estimators
We provide various XGBoost estimators for better user experience.
All of them are case-insensitive and sharing same prefix `xgboost`. They are listed below.

* xgboost.Estimator

  General estimator, with this `train.objective` should be defined explicitly.
  
* xgboost.Classifier 
    
  Estimator for classification task, works with `train.num_class`. Default value is binary classification.
  
* xgboost.BinaryClassifier 
    
  Estimator for binary classification task, the value of `train.objective` is `binary:logistic`.

* xgboost.MultiClassifier 
    
  Estimator for multi classification task, the value of `train.objective` is `multi:softprob`. It should work with `train.num_class` > 2.

* xgboost.Regressor 
    
  Estimator for regression task, the value of `train.objective` is `reg:squarederror`(`reg:linear`). 

### Columns

* Feature Columns

    For now, two kinds of feature columns are available.
    
    First one is `dense schema`, which concatenate numeric table columns transparently, such as `COLUMN f1, f2, f3, f4`.
    
    Second one is `sparse key-value schema`, which received LIBSVM style key-value string formatted like `$k1:$v1,$k2:$v2,...`.
    This schema is decorated with keyword `SPARSE`, such as `COLUMN SPARSE(col1)`.

* Label Column
  
   Following general sqlflow syntax, label clause of AntXGBoost is formatted in  `LABEL $label_col`. 

* Group Column

    In training mode, group column can be declared in a separate column clause. Group column is identified by keyword `group`, such as `COLUMN ${group_col} FOR group`.

* Weight Column
    
    As group column schema, weight column is identified by keyword `weight`, such as `COLUMN ${weight_col} FOR weight`.

* Result Columns
    
    Schema of straightforward result (class_id for classification task, score for regression task) is following general sqlflow syntax(`PREDICT ${output_table}.${result_column}`).

    In addition, we also provide supplementary information of XGBoost prediction. They can be configured with `pred.attributes`.

    * append columns

        Columns of prediction data table which need to be appended into result table, such as id_col, label_col.
        
        The syntax is `pred.append_columns = [$col1, $col2, ...]`. 
    
    * classification probability
        
        Probability of the chosen class, which only work in classification task.

        The syntax is `pred.prob_column = ${col}`.
    
    * classification detail
        A json string who holds the probability distribution of all classes, formatted like `{$class_id:$class_prob,...}`. Only work in classification task.

        The syntax is `pred.detail_column = ${col}`.
    
    * encoding of leaf indices 
        
        Predicted leaf index in each tree, they are joined orderly into a string with format `$id_1,$id_2,...`.

        The syntax is `pred.encoding_column = ${col}`.

### Attributes

There exists two kinds of attributes, `train.attributes` and `pred.attrbutes`.
`train.attributes`, which starts with prefix `train.`, only work in training mode.
`pred.attributes`, which starts with prefix `pred.`, only work in prediction mode.

All attributes are optional except `train.objective` must be defined when training with `xgboost.Estimator`.

#### Available train.attributes

* [General Params](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters)
    * train.booster
    * train.verbosity

* [Tree Booster Params](https://xgboost.readthedocs.io/en/latest/parameter.html#parameters-for-tree-booster)
    * train.eta
    * train.gamma
    * train.max_depth
    * train.min_child_weight
    * train.max_delta_step
    * train.subsample
    * train.colsample_bytree
    * train.colsample_bylevel
    * train.colsample_bynode
    * train.lambda
    * train.alpha
    * train.tree_method
    * train.sketch_eps
    * train.scale_pos_weight
    * train.grow_policy
    * train.max_leaves   
    * train.max_bin
    * train.num_parallel_tree    
 
* [Learning Task Params](https://xgboost.readthedocs.io/en/latest/parameter.html#learning-task-parameters)
    * train.objective
    * train.eval_metric
    * train.seed
    * train.num_round
        * The number of rounds for boosting
    * train.num_class
        * The number of label class in classification task
        
* AutoTrain Params
    * train.convergence_criteria
        * see supplementary for more details
    * train.auto_train 
        * see supplementary for more details

 
#### Available pred.attributes
   * pred.append_columns
   * pred.prob_column
   * pred.detail_column
   * pred.encoding_column
   

## Overall SQL Syntax for Ant-XGBoost
### Training Syntax
```sql
// standard select clause
SELECT ... FROM ${TABLE_NAME}
// train clause
TRAIN xgboost.${estimatorType}
WITH
    [optional] ${train.attributes}
    ......
    ......
COLUMN ${feature_columns}
[optional] COLUMN ${group_column} FOR group
[optional] COLUMN ${weight_column} FOR weight
LABEL ${label_column}
INTO ${model};
```
### Prediction Syntax
```sql
// standard select clause
SELECT ... FROM ${TABLE_NAME}
// pred clause
PREDICT ${output_table}.${result_column}
WITH
    [optional] ${pred.attributes}
    ......
USING ${model};
```

## Supplementary
### Generalized Early Stopping Strategy
`dmlc/xgboost` stops when no significant improvements in the recent n boosting rounds, where n is a configurable parameter.
In Ant-XGBoost, we generalize this strategy and call the new strategy convergence test. 
We keep track of the series of metric values and determine whether the series is converged or not. 
There are three main parameters to test convergence: `minNumPoints`, `n` and `c`. 
Only after the series is longer than (or equal to)  `minNumPoints`, it start to be eligible for convergence test. 
Once a series is at least `minNumPoints` long, we find the index `idx` for the best metric value so far. 
We say a series is converged if `idx + n < size * c`, where `size` is the current number of points in the series. 
The intuition is that the best metric value should be peaked (or bottomed) with a wide margin. 

With `n` and `c` we can implement complex convergence rules, but there are two common cases.
* `n > 0` and `c = 1.0`
  
   This reduces to the standard early stopping strategy that is employed by dmlc/xgboost.

* `n = 0` and `c in [0, 1]`

    For example, `n = 0` and `c = 0.8`. This means there should be at least 20% of points after the best metric value. Smaller value in `c` leads to a more conservative convergence test. This rule tests convergence in an adaptive way; for some problem the metric values are noisy and grow slowly, this rule will have better chance to find the optimal model. 

In addition, convergence test understands the optimization direction for all built-in metrics, so there is no need to set `maximize` parameter (defaults to `false`, forgetting to set this parameter often leads to strange behavior when metric value should be maximized).


### AutoTrain
With convergence test, we implement a simple `auto_train` method. There are several components in `auto_train`:
* Automatic parameter validation, setting and rewriting

    Setting the right parameters in XGBoost is not easy. For example, when working with `binary:logistic`, one should not set `num_class` to 2 (otherwise XGBoost will fail with an exception). 
    
    In Ant-XGBoost, we validate parameters to make sure all parameters are consistent with each other, e.g., `num_class = 3` and `objective = binary:logistic` will raise an exception. 
    
    In addition, we try our best to understand the input parameters from the user and automatically set or rewrite some parameters in `auto_train` mode. 
    For example, when the feature dimension is very high, building a single tree will be very inefficient,
    we will automatically set `colsample_bytree` and make sure at most 2000 features are used to build for each tree. Note that automatic parameter rewritting is only turned on in `auto_train` mode, in standard `train`, we only validate parameters and the behavior is fully controled by the user.

* automatic training

    With convergence test, number of trees in a boosted ensemble becomes a less important parameter; 
    one can always set a very large number and rely on convergence test to figure out the right number of trees. 
    The most important parameters to tune in boosted trees now become learning rate and max depth. 
    In Ant-XGBoost, we employ grid search with early stopping to efficiently search the best model structure; 
    unpromising learning rate or depth will be skipped entirely.
    
    While the current `auto_train` method is a very simple approach, we are working on better strategies to further scale up hyperparameter tuning in XGBoost training.
