# Extend the SPARSE Column Transformer to Load Sparse Vectors in Different Data Formats

# Background

In the `TO TRAIN` syntax, users can write a `COLUMN` clause to specify a table column's transformation into a model input.  Among the supported column transformers, `DENSE` and `SPARSE` are used to transform the numeric-typed values:

- The `DENSE` transformer assumes that each table cell contains a number or a dense vector.
- The `SPARSE` transformer believes that each table cell contains a sparse vector.

In the current implementation, both `DENSE` and `SPARSE` can parse a string of integers.  `DENSE` assumes that each value in the string is an element in a dense vector.  `SPARSE` thinks that each value is an element index, and all element values are 1.  For example, `DENSE` parses the string "5,6,7" into a three-dimensional dense vector [5,6,7], and `SPARSE` parses it into a sparse vector {5:1.0, 6:1.0, 7:1.0}.

Recently, users reported [a feature request](https://github.com/sql-machine-learning/sqlflow/issues/2323) out of the capability of the `SPARSE` implementation. The string-encoded sparse vector is in the key-value form of `"0:1.2 1:3.4 2:5.6"`. It represents a sparse vector `x` and `x[0] = 1.2`, `x[1] = 3.4`, `x[2] = 5.6`. In this key-value form, the whitespace is optional, and the key-value separator does not have to be a colon.

We can indeed add a new column transformer for this case. But it would make the SQLFlow APIs more complex, and disobey the principle of Occam's Razor. Therefore, we propose to extend the `SPARSE` transformer to support the new case. In the feature derivation stage, SQLFlow should infer the data format automatically, including the element separator like the comma, the key-value separator like the colon, and whether the data format is in the form of `"5,6,7"` or `"0:1.2 1:3.4 2:5.6"`.

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
- For XGBoost models: we would dump the data in the key-value form into [LibSVM format](https://xgboost.readthedocs.io/en/latest/tutorials/input_format.html) files, and then SQLFlow would load the files as `xgboost.DMatrix` for training, prediction, evaluating, and explaining.

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

Users should write `SPARSE(column_name, length)` to indicate that the column `column_name` stores the sparse data. We would detect the data format of the `column_name` in the feature derivation stage automatically.

The `length` parameter in `SPARSE` is not required. If users do not provide the `length` parameter, we will derive the dense length of the data in the feature derivation stage.
