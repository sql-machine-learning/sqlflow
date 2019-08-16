# Design: Feature Derivation

This file discusses the details and implementations of "Feature Derivation".
Please refer to [this](https://medium.com/@SQLFlow/feature-derivation-the-conversion-from-sql-data-to-tensors-833519db1467) blog to
get the background information before going on.

Target: We need to know the below two information for each column after the
feature type infer routine:

1. How to transform the column data to tensors, including `tf.SparseTensor`.
2. What type of feature column should adapt to the column and the parameters for the feature column call.

## Simpler COLUMN Clause

When we have a training table contains many columns that should be used
for training like https://www.kaggle.com/mlg-ulb/creditcardfraud, it's
not friendly if we must provide all column names in `COLUMN` clause, we'd
like to just use `COLUMN *` here.

But, for cases when the columns are of quite different data format, like:

```
data type: (float, float, float, float, csv_string_for_dense_tensor, csv_string_for_sparse_tensor, int)
column name: (a , b , c , d , e , f, label)
```

`COLUMN *` and plus the data in the table may not provide enough information we need:

- whether the `csv_string_for_dense_tensor` should going through an embedding layer
- what the original "dense shape" for the `csv_string_for_sparse_tensor` column to parse to a `SparseTensor`.

So we need SQLFlow users to provide such information in the SQL statements to make the training
network specific. We support the below SQL statements for the above case:

```
SELECT * FROM training_table
TRAIN DNNClassifier
WITH someattr=somevalue
COLUMN *,EMBEDDING(e, 128, "sum"),EMBEDDING(SPARSE(f, [1000000]), 512, "sum")
LABEL label
INTO my_model_file;
```

Note that the above SQL have the same meaning to:

```
SELECT * FROM training_table
TRAIN DNNClassifier
WITH someattr=somevalue
COLUMN a, b, c, d ,EMBEDDING(DENSE(e, [64]), 128, "sum"),EMBEDDING(SPARSE(f, [1000000]), 512, "sum")
LABEL label
INTO my_model_file;
```

Since we can not get the original "dense shape" if the CSV column represents a sparse tensor,
the shape must be specified in the SQL statement. But if the column represents a "dense tensor"
we can get the shape by reading some of the values and confirm the shape is the same.

For CSV values, we can also infer the tensor data type by reading some of the training data, whether
it's int value or float value. Note we can always parse float values to `float32` but not `float64`
since `float32` seems enough for most cases.

## The Feature Derivation Routine

We need to `SELECT` part of the training data, like 1000 rows and go through the below routine:

1. If the column data type is numeric: int, bigint, float, double, can directly parse to a tensor of shape `[1]`.
2. If the column data type is string: VARCHAR or TEXT:
   1. If the string is not one of the supported serialized format (only support CSV currently):
      1. If all the rows of the column's string data can be parsed to a float or int value,
         treat it as a tensor of shape `[1]`.
      2. If the int value of the above steps is very large, use `categorical_column_with_hash_bucket`.
      3. The string value can not be parsed to int or float, treat it as enum type and use
         `categorical columns` to process the string to tensors.
      4. If the enum values in the above step have very little in common (like only 5% of the data appeared twice or more), use `categorical_column_with_hash_bucket`.
    2. If the string is of CSV format, try to infer the inner data type by reading some data, if
       float value presents, then the `dtype` should be `float32`
       1. If all rows for this column have the same number of values in the CSV, parse the column to a "dense" tensor.
       2. If the rows contain CSV data of different length, then:
          1. Check if the column is defined in SQL statement a `SPARSE` column, if not, throw an error.
          2. If the current column is defined as a `SPARSE` column, use the "dense shape" to parse the column data to a `tf.SparseTensor`.

After going through the above "routine" we can be sure how to parse the data for each column and
what feature column to use. Also, we can add support more serialized format in additional to CSV,
like JSON or protobuf.
