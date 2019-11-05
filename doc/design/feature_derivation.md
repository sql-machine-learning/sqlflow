# Feature Derivation

This file discusses the details and implementations of "Feature Derivation".
Please refer to [this](https://medium.com/@SQLFlow/feature-derivation-the-conversion-from-sql-data-to-tensors-833519db1467) blog to
get the background information before going on.

Target: We need to know the below two information for each column after the
feature type infer routine:

1. How to transform the column data to tensors, including `tf.SparseTensor`.
2. What type of feature column should adapt to the column and the parameters
   for the feature column call.

## Simpler COLUMN Clause

We assume all the selected columns will be used as either `COLUMN` or `LABEL`.

When we have a training table contains many columns that should be used
for training like https://www.kaggle.com/mlg-ulb/creditcardfraud, it's
not friendly if we must provide all column names in `COLUMN` clause.
Since we'd like to use all columns, when we write `SELECT *` then we can
assume that we are using all columns to train and no longer need to write
`COLUMN` anymore:

```sql
SELECT * FROM creditcardfraud
TO TRAIN DNNClassifier
LABEL class
INTO my_model_name;
```

We assume all the selected columns without specific annotations will be [derived automatically](#the-feature-derivation-routine).

For columns that may need to do preprocessing, we can add those preprocessing
descriptions in the `COLUMN` clause. For the credit card fraud dataset, assume
only the column `time` should be processed use a function before feed to the model, so
the SQL statement should look like:

```sql
SELECT * FROM creditcardfraud
TO TRAIN DNNClassifier
COLUMN YOUR_NORMALIZE_FUNC(time)
LABEL class
INTO my_model_name;
```

For more complex cases when columns are of quite different data format, like:

| column name | data type |
| ----------- | --------- |
| a | float |
| b | float |
| c | string (csv as a dense tensor) |
| d | string (csv as a sparse tensor) |
| label | int |

If the column represents a "dense tensor", we can get the shape by reading
some of the values and confirm the shapes are the same.

While we can not infer the actual "dense shape" by reading the data if the CSV
string column represents a sparse tensor, nor whether the column should use a
embedding feature column. So, these information must be provided by the SQL
statement, the SQL statement for the above case should be like:

```sql
SELECT * FROM training_table
TO TRAIN DNNClassifier
COLUMN EMBEDDING(c, 128, "sum"),
       EMBEDDING(SPARSE(d, [1000000]), 512, "sum")
LABEL label
INTO my_model_name;
```

You can also write the full description of every column like below:

```sql
SELECT a, b, c, d, label FROM training_table
TO TRAIN DNNClassifier
COLUMN a, b,
       EMBEDDING(DENSE(c, [64]), 128, "sum"),
       EMBEDDING(SPARSE(d, [1000000]), 512, "sum")
LABEL label
INTO my_model_file;
```

For CSV values, we also need to infer the tensor data type by reading some of
the training data, whether it's int value or float value. Note that we always parse
float values to `float32` but not `float64` since `float32` seems enough for most cases.

## The Feature Derivation Routine

We need to `SELECT` part of the training data, like 1000 rows and go through the below routine:

1. If the column processor is specified in the `COLUMN` clause, parse the column as described.
   try to infer the inner data type by reading some data, if float value presents, then the
   `dtype` should be `float32`.
2. If the column data type is numeric: int, bigint, float, double, can directly parse to a tensor of shape `[1]`.
3. If the column data type is string: VARCHAR or TEXT:
   1. If the string is not one of the supported serialized format (only support CSV currently):
      1. If all the rows of the column's string data can be parsed to a float or int value,
         treat it as a tensor of shape `[1]`.
      2. If the int value of the above steps is very large, use `categorical_column_with_hash_bucket`.
      3. The string value can not be parsed to int or float, treat it as enum type and use
         `categorical columns` to process the string to tensors.
      4. If the enum values in the above step have very little in common (like only 5% of the data appeared twice or more), use `categorical_column_with_hash_bucket`.
    2. If the string is of CSV format:
       1. If already appeared in `COLUMN` clause, then continue.
       2. If all rows for this column have the same number of values in the CSV, parse the column to a "dense" tensor, use this dense tensor directly as model input.
       3. If the rows contain CSV data of different length, then return a parsing error to the
          client and top.

After going through the above "routine" we can be sure how to parse the data for each column and
what feature column to use. Also, we can add support more serialized format in additional to CSV,
like JSON or protobuf.
