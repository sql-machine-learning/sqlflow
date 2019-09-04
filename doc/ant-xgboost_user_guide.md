# _user guide:_ Ant-XGBoost on sqlflow

## Overview

[Ant-XGBoost](https://github.com/alipay/ant-xgboost) is fork of [dmlc/xgboost](https://github.com/dmlc/xgboost), which is maintained by active contributors of dmlc/xgboost in Alipay Inc.

Ant-XGBoost extends `dmlc/xgboost` with the capability of running on Kubernetes and automatic hyper-parameter estimation. 
In particular, Ant-XGBoost includes `auto_train` methods for automatic training and introduces an additional parameter `convergence_criteria` for generalized early stopping strategy.

## Tutorial
We provide an [interactive tutorial](../example/jupyter/tutorial_antxgb.ipynb) via jupyter notebook, which can be run out-of-the-box in [sqlflow playground](https://play.sqlflow.org).
If you want to run it locally, you need to install sqlflow first. You can learn how to install sqlflow at [here](../doc/installation.md).

## Concepts
### Estimators
We provide various XGBoost estimators for better user experience.
All of them are case-insensitive and sharing same prefix `xgboost`. They are listed below.

* xgboost.Estimator

  General estimator, with this `train.objective` should be defined explicitly.
  
* xgboost.Classifier 
    
  Estimator for classification task, works with `train.num_class`. Default value is binary classification.
  
* xgboost.BinaryClassifier 
    
  Estimator for binary classification task, set `train.objective` to `binary:logistic`.

* xgboost.MultiClassifier 
    
  Estimator for multi classification task, set `train.objective` to `multi:softprob`, should work with `train.num_class` > 2.

* xgboost.Regressor 
    
  Estimator for regression task, set `train.objective` to `reg:squarederror`(`reg:linear`). 

<br>

### Columns

#### Feature Columns
For now, two feature column schemas are available.

First one is `dense schema`, which concatenate numeric table columns transparently, such as `COLUMN f1, f2, f3, f4`.

Second one is `sparse key-value schema`, which received string sparse feature formatted like `$k1:$v1,$k2:$v2,...`.
This schema is decorated with keyword `SPARSE`, such as `COLUMN SPARSE(col1)`.

#### Label Column
Following general sqlflow syntax, label clause of AntXGBoost is formatted in  `LABEL $label_col`. 

#### Group Column
In training mode, group column can be declared in a separate column clause. Group column is identified by keyword `group`, such as `COLUMN ${group_col} FOR group`.

#### Weight Column
As group column schema, weight column is identified by keyword `weight`, such as `COLUMN ${weight_col} FOR weight`.

#### Result Columns
Schema of straightforward result (class_id for classification task, score for regression task) is following general sqlflow syntax(`PREDICT ${output_table}.${result_column}`).

In addition, we also provide supplementary information of XGBoost prediction. They can be configured with `pred.attributes`.

##### append columns
Columns of prediction data table which need to be appended into result table, such as id_col, label_col.

The syntax is `pred.append_columns = [$col1, $col2, ...]`. 
##### classification probability
Probability of the chosen class, which only work in classification task.

The syntax is `pred.prob_column = ${col}`.
##### classification detail
A json string who holds the probability distribution of all classes, formatted like `{$class_id:$class_prob,...}`. Only work in classification task.

The syntax is `pred.detail_column = ${col}`.
##### encoding of leaf indices 
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
    * train.auto_train 
 
 
#### Available pred.attributes
   * pred.append_columns
   * pred.prob_column
   * pred.detail_column
   * pred.encoding_column
   

## Overall SQL Syntax
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
