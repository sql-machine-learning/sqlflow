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

You wanna train a DNNClassifier, which has 2 hiddens layers and each layer has 10 hidden units, and save the trained model into table `sqlflow_models.my_dnn_model` for later prediction use.

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

## Syntax

### Select clause

`SELECT * FROM iris.train` in the overview example is considered as the *select clause*. It describes the data retrieved from a particular table.

```SQL
SELECT select_expr [, select_expr ...]
FROM table_references
  [WHERE where_condition]
  [LIMIT row_count]
```

Equivalent to [ANSI SQL Standards](https://www.whoishostingthis.com/resources/ansi-sql-standards/),
- Each *select_expr* indicates a column that you want to retrieve. There must be at least one *select_expr*.
- *table_references* indicates the table or tables from which to retrieve rows.
- *where_condition* is an expression that evaluates to true for each row to be selected.
- *row_count* indicates the maximum number of rows to be retrieved.

For example, if you wanna fast prototype a binary classifier on a small set of sample data, you can write

```SQL
SELECT *
FROM iris.train
WHERE class = 0 OR class = 1
LIMIT 1000
TRAIN ...
```

### Train clause

`TRAIN DNNClassifer WITH hidden_units = [10, 10], n_classes = 3, EPOCHS = 10` in the overview example is considered as the *train clause*. It describes the specific model type and the way the model is trained.

```SQL
TRAIN model_identifier
WITH
  model_attr_expr [, model_attr_expr ...]
  [, train_attr_expr ...]
```

- *model_identifier* indicates the model type. e.g. `DNNClassifier`. Please refer to [Models](#Models) for a complete list of supported models.
- *model_attr_expr* indicates the model attribute. e.g. `n_classes = 3`. Please refer to [Models](#Models) for details.
- *train_attr_expr* indicates the training attribute. e.g. `EPOCHS = 10`. Please refer to [Hyperparameters](#Hyperparameters) for details.

For example, if you wanna train a DNNClassifier, which has 2 hiddens layers and each layer has 10 hidden units, with 10 epochs, you can write

```SQL
SELECT ...
TRAIN DNNClassifer
WITH
  hidden_units = [10, 10],
  n_classes = 3,
  EPOCHS = 10
...
```

### Column clause

### Label clause

### Save clause

## Feature columns

SQLFlow supports various feature columns to preprocess raw data. Here is a growing list.

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

```SQL
NUMERIC(field, n[, delimiter=comma])
/*
NUMERIC converts a delimiter separated string to a n dimensional Tensor
    field:
        field name of standard select result.
        e.g. dense, column1.
    dimension:
        tensor dimension.
        e.g. 12, [3,4].
    delimiter:
        delimiter.
        default: comma.

Example:
    NUMERIC(dense, 3). "0.2,1.7,0.6" => Tensor(0.2, 1.7, 0.6)

Error:
    Invalid field type. field type has to be string/varchar[n]
    Invalid dimension. e.g. convert "0.2,1.7,0.6" to dimension 2.
    */
```

### CATEGORY_ID

### SEQ_CATEGORY_ID

### EMBEDDING

### ONE_HOT

## Models

A detailed explanation of the train clause.

### What if some models have multiple inputs?

Wide and deep example.

## Hyperparameters

A detailed explanation of the train clause. `BATCHSIZE`, `EPOCHS` etc..

## How can I store the model?

1. Save to the table
1. Save to the file system

