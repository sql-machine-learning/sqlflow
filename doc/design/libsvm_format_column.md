# Extend the SPARSE Column Transformer to Load Sparse Vectors in Different Data Formats

# Background

In the `TO TRAIN` syntax, users can write a `COLUMN` clause to specify the transformation of a table column into a model input.  Among the supported column transformers, `DENSE` and `SPARSE` are used to transform the numeric-typed values:

- The `DENSE` assumes that each table cell contains a number or a dense vector.
- The `SPARSE` assumes that each table cell contains a sparse vector.

In the current implementation, both `DENSE` and `SPARSE` have supported the string-encoded dense vector in the form of `"5,6,7"`.

- If `"5,6,7"` represents a dense vector `x`, `x = [5, 6, 7]`. In this case, the `COLUMN` clause is `DENSE(column_name, 3)`, where 3 is the element number of the dense vector.
- If `"5,6,7"` represents a sparse vector `x`, the vector `x` only contains 0 or 1, and `x[5] = x[6] = x[7] = 1`. The other values of `x` are all zeros. In this case, the `COLUMN` clause is `SPARSE(column_name, length)`, where the `length` is the element number of the corresponding dense vector.

Recently, users reported [a feature request](https://github.com/sql-machine-learning/sqlflow/issues/2323) out of the capability of the `SPARSE` implementation. The string-encoded sparse vector is in the key-value form of `"0:1.2 1:3.4 2:5.6"`. It represents a sparse vector `x` and `x[0] = 1.2`, `x[1] = 3.4`, `x[2] = 5.6`. In this key-value form, the whitespace is optional, and the key-value separator does not have to be a colon.

We can indeed add a new column transformer for the new case. But it would make the SQLFlow APIs more complex, and disobey the principle of Occam's Razor. Therefore, we propose to extend the `SPARSE` transformer to support the new case. In the feature derivation stage, SQLFlow should infer the data format automatically, including the element separator like the comma, the key-value separator like the colon, and whether the data format is in the form of `"5,6,7"` or `"0:1.2 1:3.4 2:5.6"`.

## Proposed Design

We would add a field named `format` in `FieldDesc`. It may be `csv`(`"5,6,7"`), `kv` (`"0:1.2 1:3.4 2:5.6"`), or other data format we would support in the future.

```go
type FieldDesc struct {
   Name      string
   DType     int
   Delimiter string
   Shape     []int
   IsSparse  bool
   Format    string  // indicates the data format
   ...
}
```

In the feature derivation stage, we can use a regular expression to infer the data format. To avoid too much regular expression matching, this matching would be done only once when inferring the first row of the fetched samples. That is to say (in pseudo codes):

```go
func InferFeatureColumns() {
    rows := FetchSamples()
    rowCount := 0
    for rows.Next() {
        if rowCount == 0 {
            format := inferDataFormat(rows.Value()) // Use regular expression to infer the data format
            if format == "kv" {
                // Fill FieldDesc info when the data format is in the key-value form
                // For example: FieldDesc.Format = "kv", FieldDesc.IsSparse = true, etc.
                ...
            } else { // other supported data format
                ...
            }
        }
        rowCount ++
    }
}
```

In the Python code generation, we would use the data format information inferred in the feature derivation stage.

- For TensorFlow models: we would convert the data in the key-value form into `tensorflow.SparseTensor` for training, prediction, evaluating, and explaining.
- For XGBoost models: we would dump the data in the key-value form into [LibSVM format](https://xgboost.readthedocs.io/en/latest/tutorials/input_format.html) files, which would be loaded as `xgboost.DMatrix` for training, prediction, evaluating, and explaining.

## SQL Statement Example

The SQL statement to load the sparse data from the table column is:

```sql
SELECT * FROM train_table
TO TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN SPARSE(column_name, 10000)
LABEL label
INTO result_table;
```

where users should write `SPARSE(column_name, length)` to indicate that the column `column_name` stores the sparse data. We would detect the data format of the `column_name` in the feature derivation stage automatically.

The `length` parameter in `SPARSE` is not required. If the `length` is not provided, we would derive the dense length of the data in the feature derivation stage.
