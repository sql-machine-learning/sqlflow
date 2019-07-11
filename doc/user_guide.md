# SQLFlow User Guide

SQLFlow is a bridge that connects a SQL engine (e.g. MySQL, Hive, or MaxCompute) and TensorFlow and other machine learning toolkits.  SQLFlow extends the SQL syntax to enable model training and inference.

## Overview

Say you have your [iris flower data set](https://en.wikipedia.org/wiki/Iris_flower_data_set) stored in table `iris.train`. The first four columns(petal_length, petal_width, sepal_length, sepal_width) are the features. And the last column(class) is the label.

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

You wanna train a DNNClassifier, which has 2 hidden layers and each layer has 10 hidden units, and save the trained model into table `sqlflow_models.my_dnn_model` for later prediction use.

Instead of writing a Python program with a lot of boilerplate code, you can simply write the following statement in SQLFlow.

```SQL
SELECT * FROM iris.train
TRAIN DNNClassifer
WITH hidden_units = [10, 10], n_classes = 3, EPOCHS = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

SQLFlow will parse the statement and transpile it to an equivalent Python program for you.

![](figures/user_overview.png)

## Syntax

A SQLFlow training statement consists of a sequence of select, train, column, label and into clauses.

### Select clause

The *select clause* describes the data retrieved from a particular table, e.g. `SELECT * FROM iris.train`.

```SQL
SELECT select_expr [, select_expr ...]
FROM table_references
  [WHERE where_condition]
  [LIMIT row_count]
```

Equivalent to [ANSI SQL Standards](https://www.whoishostingthis.com/resources/ansi-sql-standards/),
- Each *select_expr* indicates a column that you want to retrieve. There must be at least one *select_expr*.
- *table_references* indicates the table from which to retrieve rows.
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

The *train clause* describes the specific model type and the way the model is trained, e.g. `TRAIN DNNClassifer WITH hidden_units = [10, 10], n_classes = 3, EPOCHS = 10`.

```SQL
TRAIN model_identifier
WITH
  model_attr_expr [, model_attr_expr ...]
  [, train_attr_expr ...]
```

- *model_identifier* indicates the model type. e.g. `DNNClassifier`. Please refer to [Models](#models) for a complete list of supported models.
- *model_attr_expr* indicates the model attribute. e.g. `n_classes = 3`. Please refer to [Models](#models) for details.
- *train_attr_expr* indicates the training attribute. e.g. `EPOCHS = 10`. Please refer to [Hyperparameters](#hyperparameters) for details.

For example, if you wanna train a DNNClassifier, which has 2 hidden layers and each layer has 10 hidden units, with 10 epochs, you can write

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

The *column clause* indicates the field name to be used as training features, along with their preprocessing if needed, e.g. `COLUMN sepal_length, sepal_width, petal_length, petal_width`.

```SQL
COLUMN column_expr [, column_expr ...]
  | COLUMN column_expr [, column_expr ...] FOR column_name
    [COLUMN column_expr [, column_expr ...] FOR column_name ...]
```

- *column_expr* indicates the field name and the preprocessing method on the field content. e.g. `sepal_length`, `NUMERIC(dense, 3)`. Please refer to [Feature columns](#feature-columns) for preprocessing details.
- *column_name* indicates the feature column names for the model inputs. Some models such as [DNNLinearCombinedClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNLinearCombinedClassifier) have`linear_feature_columns` and `dnn_feature_columns` as feature column input.

For example, if you wanna use field `sepal_length`, `sepal_width`, `petal_length`, and `petal_width` as the features, without any preprocessing on the field content, you can write

```SQL
SELECT ...
TRAIN ...
COLUMN sepal_length, sepal_width, petal_length, petal_width
...
```

### Label clause

The *label clause* indicates the field name to be used as the training label, along with their preprocessing if needed, e.g. `LABEL class`.

```SQL
LABEL label_expr
```

- *label_expr* indicates the field name and the preprocessing method on the field content. e.g. `class`.

Note: some field name may look like SQLFlow keywords. For example, the table may contain a field named label. You can use double quotes around the name `LABEL "label"` to work around the parsing error.

### Into clause

The *into clause* indicates the table name to save the trained model

```SQL
INTO table_references
```

- *table_references* indicates the table to save the trained model. e.g. `sqlflow_model.my_dnn_model`.

Note: SQLFlow team is actively working on supporting saving model to third-party storage services such as AWS S3, Google Storage and Alibaba OSS.

## Feature columns

SQLFlow supports various feature columns to preprocess raw data. Here is a growing list of supported feature columns.

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
    <td>NUMERIC(field, n[, delimiter])</td>
    <td>string/varchar[n]</td>
    <td>"0.2,1.7,0.6"</td>
  </tr>
  <tr>
    <td>CATEGORY_ID</td>
    <td>CATEGORY_ID(field, n[, delimiter])</td>
    <td>string/varchar[n]</td>
    <td>"66,67,42,68,48,69,70"</td>
  </tr>
  <tr>
    <td>SEQ_CATEGORY_ID</td>
    <td>SEQ_CATEGORY_ID(field, n[, delimiter])</td>
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

### NUMERIC

```SQL
NUMERIC(field, n[, delimiter=comma])
/*
NUMERIC converts a delimiter separated string to a n dimensional Tensor
    field:
        A string specifying the field name of the standard select result.
        e.g. dense, column1.
    n:
        An integer specifying the tensor dimension.
        e.g. 12, [3,4].
    delimiter:
        A string specifying the delimiter.
        default: comma.

Example:
    NUMERIC(dense, 3). "0.2,1.7,0.6" => Tensor(0.2, 1.7, 0.6)

Error:
    Invalid field type. field type has to be string/varchar[n]
    Invalid dimension. e.g. convert "0.2,1.7,0.6" to dimension 2.
*/
```

### CATEGORY_ID

Implements [tf.feature_column.categorical_column_with_identity](https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_identity).

```SQL
CATEGORY_ID(field, n[, delimiter=comma])
/*
CATEGORY_ID splits the input field by delimiter and returns identiy values
    field:
        A string specifying the field name of the standard select result.
        e.g. title, id, column1.
    n:
        An integer specifying the number of buckets
        e.g. 12, 10000.
    delimiter:
        A string specifying the delimiter.
        default: comma.

Example:
    CATEGORY_ID(title, 100). "1,2,3,4" => Tensor(1, 2, 3, 4)

Error:
    Invalid field type. field type has to be string/varchar[n]
*/
```

### SEQ_CATEGORY_ID

Implements [tf.feature_column.sequence_categorical_column_with_identity](https://www.tensorflow.org/api_docs/python/tf/feature_column/sequence_categorical_column_with_identity).

```SQL
SEQ_CATEGORY_ID(field, n[, delimiter=comma])
/*
SEQ_CATEGORY_ID splits the input field by delimiter and returns identiy values
    field:
        A string specifying the field name of the standard select result.
        e.g. title, id, column1.
    n:
        An integer specifying the number of buckets
        e.g. 12, 10000.
    delimiter:
        A string specifying the delimiter.
        default: comma.

Example:
    SEQ_CATEGORY_ID(title, 100). "1,2,3,4" => Tensor(1, 2, 3, 4)

Error:
    Invalid field type. field type has to be string/varchar[n]
*/
```

### EMBEDDING

Implements [tf.feature_column.embedding_column](https://www.tensorflow.org/api_docs/python/tf/feature_column/embedding_column).

```SQL
EMBEDDING(category_column, n[, combiner])
/*
EMBEDDING converts a delimiter separated string to an n-dimensional Tensor
    category_column:
        A category column created by CATEGORY_ID*
        e.g. CATEGORY_ID(title, 100).
    n:
        An integer specifying the dimension of the embedding, must be > 0.
        e.g. 12, 100.
    combiner:
        A string specifying how to reduce if there are multiple entries in a single row.

Example:
    EMBEDDING(CATEGORY_ID(news_title,16000,COMMA), 3, mean). "1,2,3" => Tensor(0.2, 1.7, 0.6)
*/
```

## Models

SQLFlow supports various TensorFlow premade estimators.

### DNNClassifer

```SQL
TRAIN DNNClassifier
WITH
    hidden_units=[10,10],
    n_classes=2,
    optimizer='Adagrad',
    batch_norm=False
```

### DNNLinearCombinedClassifier

```SQL
TRAIN DNNLinearCombinedClassifier
WITH
    linear_optimizer='Ftrl',
    dnn_optimizer='Adagrad',
    dnn_hidden_units=None,
    n_classes=2,
    batch_norm=False,
    linear_sparse_combiner='sum'
COLUMN ... FOR linear_feature_columns
COLUMN ... FOR dnn_feature_columns
```

## Hyperparameters

SQLFlow supports various configurable training hyperparameters.

1. `BATCHSIZE`. Default 1.
1. `EPOCHS` . Default 1.
