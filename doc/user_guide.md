# SQLFlow User Guide

SQLFlow is a bridge that connects a SQL engine (e.g. MySQL, Hive, or Maxcompute) and TensorFlow and other machine learning toolkits.  SQLFlow extends the SQL syntax to enable model training and inference.

## Overview

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

```SQL
-- select clause
SELECT * FROM iris.train
-- train clause
TRAIN DNNClassifer
WITH n_classes, EPOCHS=10
-- column clause
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
-- save clause
INTO sqlflow_models.my_dnn_model;
```

![](figures/user_overview.png)

## What kind of select statement can I use?

A detailed explanation of the select clause.

## What type of preprocessing can I apply to selected data?

A detailed explanation of the column clause.

<table>
  <tr>
    <th>feature column type</th>
    <th>table field type</th>
    <th>example</th>
  </tr>
  <tr>
    <td>/</td>
    <td>int/float/double</td>
    <td>1.3</td>
  </tr>
  <tr>
    <td>NUMERIC</td>
    <td>string/varchar[n]</td>
    <td>"0.2,1.7,0.6"</td>
  </tr>
  <tr>
    <td>...</td>
    <td></td>
    <td></td>
  </tr>
</table>

### NUMERIC

```
NUMERIC(field_name, dimension, [delimiter])
    field_name
    dimension: e.g. 12, [3,4]
    delimiter: e.g. comma
```

Before: "0.2,1.7,0.6".
After: Tensor(0.2, 1.7, 0.6)

### ONE_HOT

...

## What kind of model can I use?

A detailed explanation of the train clause.

### What if some models have multiple inputs?

Wide and deep example.

## How do I adjust the hyperparameter?

A detailed explanation of the train clause. `BATCHSIZE`, `EPOCHS` etc..

## How can I store the model?

1. Save to the table
1. Save to the file system

