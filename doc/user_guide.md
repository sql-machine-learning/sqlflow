# SQLFlow User Guide

SQLFlow is a bridge that connects a SQL engine (e.g. MySQL, Hive, or Maxcompute) and TensorFlow and other machine learning toolkits.  SQLFlow extends the SQL syntax to enable model training and inference.

## Overview

Say you have your data in table `iris.train`. The first four columns is the features and the last column is the label.

<table>
  <tr>
    <th colspan="5">iris.train</th>
  </tr>
  <tr>
    <td>sepal_length</td>
    <td>sepal_width</td>
    <td>petal_length</td>
    <td>petal_width</td>
    <td>class</td>
  </tr>
  <tr>
    <td>6.4</td>
    <td>2.8</td>
    <td>5.6</td>
    <td>2.2</td>
    <td>2</td>
  </tr>
  <tr>
    <td>5.0</td>
    <td>2.3</td>
    <td>3.3</td>
    <td>1.0</td>
    <td>1</td>
  </tr>
  <tr>
    <td>...</td>
    <td>...</td>
    <td>...</td>
    <td>...</td>
    <td>...</td>
  </tr>
</table>

You wanna train a DNNClassifier which has 2 hiddens layers and each layer has 10 hidden units. At the end of the training, you wanna save the trained DNNClassifier into a table named `sqlflow_models.my_dnn_model` for later prediction use.

Instead of writting a Python program with a lot of boilerplate code, you can simply write the following statement in SQLFlow.

```SQL
SELECT * FROM iris.train
TRAIN DNNClassifer
WITH hidden_units = [10, 10], n_classes = 3, EPOCHS = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

SQLFlow will parse the statement and transpile it to a equivalent Python program for you.

![](figures/user_overview.png)

## What kind of standard select statement can I use?

We consider `SELECT * FROM iris.train` in the overview example as the *standard select*.

Currently SQLFlow supports *standard select* syntax as

```SQL
SELECT select_expr [, select_expr ...]
FROM table_references
  [WHERE where_condition]
  [LIMIT row_count]
```

And the team is working on supporting arbitary select statements.

## What type of preprocessing can I apply to selected data?

A detailed explanation of the column clause.

<table>
  <tr>
    <th>feature column type</th>
    <th>usage</th>
    <th>field type</th>
    <th>example</th>
  </tr>
  <tr>
    <td>X</td>
    <td>field</td>
    <td>int/float/double</td>
    <td>3.14</td>
  </tr>
  <tr>
    <td>NUMERIC</td>
    <td>NUMERIC(field, dimension[, delimiter])</td>
    <td>string/varchar[n]</td>
    <td>"0.2,1.7,0.6"</td>
  </tr>
  <tr>
    <td>CATEGORY_ID</td>
    <td>CATEGORY_ID(field, dimension[, delimiter])</td>
    <td>string/varchar[n]</td>
    <td>"66,67,42,68,48,69,70"</td>
  </tr>
  <tr>
    <td>SEQ_CATEGORY_ID</td>
    <td>SEQ_CATEGORY_ID(field, dimension[, delimiter])</td>
    <td>string/varchar[n]</td>
    <td>"20,48,80,81,82,0,0,0,0"</td>
  </tr>
  <tr>
    <td>EMBEDDING</td>
    <td>EMBEDDING(category_column, dimension[, combiner])</td>
    <td>X</td>
    <td>X</td>
  </tr>
</table>

### Plain



### NUMERIC

```
NUMERIC(field_name, dimension[, delimiter])
    field_name: e.g. dense, column1
    dimension: e.g. 12, [3,4]
    delimiter: default comma
```

`NUMERIC` column converts a delimiter separated string to a Tensor, e.g. `"0.2,1.7,0.6" => Tensor(0.2, 1.7, 0.6)`.

### CATEGORY_ID

### SEQ_CATEGORY_ID

### EMBEDDING

### ONE_HOT

## What kind of model can I use?

A detailed explanation of the train clause.

### What if some models have multiple inputs?

Wide and deep example.

## How do I adjust the hyperparameter?

A detailed explanation of the train clause. `BATCHSIZE`, `EPOCHS` etc..

## How can I store the model?

1. Save to the table
1. Save to the file system

