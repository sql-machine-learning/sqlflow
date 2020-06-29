# Support LibSVM format on SQLFlow

## Background

SQLFlow extends SQL syntax to support End-to-End machine learning pipeline. 
However, due to the variety of the data format (like CSV, LibSVM, etc.), 
data pre-processing is usually a tough task for users. Users have to speed lots of time 
to transform the raw inputs into the data format which the machine learning model accepts.
SQLFlow should do some automatic data pre-processing to make End-to-End machine learning pipeline 
easier.

LibSVM is a commonly used data format in machine learning, and it is officially supported in many machine learning frameworks, 
such as [XGBoost](https://xgboost.readthedocs.io/en/latest/tutorials/input_format.html) and 
[SkLearn](https://scikit-learn.org/stable/modules/generated/sklearn.datasets.load_svmlight_file.html). There have been 
[some issues](https://github.com/sql-machine-learning/sqlflow/issues/2323) that require SQLFlow should support LibSVM format column. 
Therefore, SQLFlow should support to process LibSVM format to simplify the End-to-End machine learning pipeline.

LibSVM format is more portable than CSV format when storing sparse data. Each line of the LibSVM file is like:

```
<label> <index>:<value> <index>:<value> ...
```

where:

- `<label>` is optional (for example, `<label>` is optional when predicting). 
- `<index>:<value>` indicates each non-empty index-value pair in the sparse data. There may be many index-value pairs in each line, 
and the pair number may be different in different lines.


There are 2 ways in SQLFlow to support the column which is in LibSVM format:

- `TO RUN` statements. It can support any user-defined data pre-processing. We can release a docker image to convert the sparse
LibSVM data to a dense Tensor, create columns for each element of the dense Tensor, and write the data into the result table. 
However, if the LibSVM data is highly sparse, it would create too many columns in the result table, and consume lots of time to 
do the transformation.
- `COLUMN` clauses. `COLUMN` clause supports some commonly used feature columns, like `NUMERIC`, `CATEGORY_ID`, `EMBEDDING`, etc.
If we want to support LibSVM data format, where would be 2 optional methods:
    - Add a new `COLUMN` clause, like `LIBSVM` or `NUMERIC_KV`. It introduces complexity, and disobeys Occam's Razor principle.
    - Extend `NUMERIC` column to support LibSVM format. Currently, we have supported CSV format implicitly: in feature derivation stage, 
    we would infer whether the column data is CSV format (see [here](https://github.com/sql-machine-learning/sqlflow/blob/3b70a0599beef573cd99f15dd41cc0a194634b75/pkg/ir/derivation.go#L146)),
    and convert the CSV format value into Tensor in Python side (see [here](https://github.com/sql-machine-learning/sqlflow/blob/develop/python/sqlflow_submitter/db.py#L159)). 
    We can do the same implicit feature derivation to LibSVM format as CSV format.
    
In conclusion, we would choose to extend `NUMERIC` column to support LibSVM format column in this design.

## Only support LibSVM data without label value

There are 2 kinds of LibSVM data format: with label value and without label value. In this design, we only support LibSVM format without label value, it is because:

- The actual label value used for training would be confused if the LibSVM data format column has label value.
    - Users may choose the label in LibSVM data as the training label, but we have not supported to choose partial data of a column as training label yet.
    - Users may choose another column as the training label, which is different with the label value in LibSVM data format column.
- If the LibSVM data contains label value, users can easily split the label into a separate column. For example, the SQL statements in MySQL is like:
    ```sql
    CREATE TABLE split_table AS 
    SELECT 
    SUBSTRING_INDEX(libsvm_column_with_label, ' ', 1) label, 
    TRIM(SUBSTR(libsvm_column_with_label, LOCATE(' ', libsvm_column_with_label))) libsvm_column_without_label
    FROM source_table;
    ```
    
Therefore, we choose to support LibSVM format without label only in this design, and suggest users to separate the label value using the SQL statement above
if the LibSVM data contains label value.

## Design

### Add format field in `FieldDesc`

Add a field named `format` in `FieldDesc`. It may be `csv`, `libsvm` or other data format we would support in the future.

```go
type FieldDesc struct {
	Name      string
	DType     int
	Delimiter string
	Shape     []int
	IsSparse  bool
	Format    string `json:"format"`    // indicates the data format
	...
}
```

### Implicit data format derivation in feature derivation stage

In feature derivation stage, we can use a regular expression to check whether the input data format is LibSVM. 
To avoid too many regular expression matching, this checking would be done only once when inferring the first row 
of the fetched samples in feature derivation stage. That is to say (in pseudo code):

```go
func InferFeatureColumns() {
    rows := FetchSamples()
    rowCount := 0
    for rows.Next() {
        if rowCount == 0 {
            format := inferDataFormat(rows.Value()) // Use regular expression to infer the data format
            if format == "libsvm" {
                // Fill FieldDesc info when the data format is LibSVM
                // For example: FieldDesc.Format = "libsvm", FieldDesc.IsSparse = true, etc.
                ...
            }
        }
        rowCount ++
    }
}
```

### Convert the LibSVM data to sparse Tensor in Python data generator

Whether the column is in LibSVM format is inferred in feature derivation stage, and the data format information would be 
used in Python codegen. In Python side, we would first read the raw data from the database, and then convert the LibSVM 
column into sparse Tensor for training, prediction, evaluating and explaining.
