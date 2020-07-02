# Load Sparse Tensors From LibSVM Format Column by SPARSE Column Clause in SQLFlow

## Definitions

### LibSVM Format File

A **LibSVM format file** is a text file storing sparse data, in which each line is like:

```
<label> <index>:<value> <index>:<value> ...
```

where:

- `<label>` is the label value of the sample and it is optional. For example, `<label>` is optional when predicting. 
- `<index>:<value>` indicates each non-zero index-value pair in the sparse data. There may be many index-value pairs in each line, and the pair number may be different in different lines.

For example, the text line `0 1:2 3:4 5:6` in a LibSVM format file indicates that:

- there is a sparse sample `x`, and `x[1]=2`, `x[3]=4`, `x[5]=6`, and the other values of sample `x` are zeros.
- the label corresponding to the sample `x` is 0.

This kind of file was firstly introduced in [LibSVM](https://www.csie.ntu.edu.tw/~cjlin/libsvm), a library for Support Vector Machines algorithms. After that, this kind of file is widely used in many other machine learning frameworks. For example, [Spark MLLib](https://spark.apache.org/docs/1.0.2/api/python/pyspark.mllib.util.MLUtils-class.html), [XGBoost](https://xgboost.readthedocs.io/en/latest/tutorials/input_format.html) and [SkLearn](https://scikit-learn.org/stable/modules/generated/sklearn.datasets.load_svmlight_file.html) officially support loading data from this kind of file for training and prediction.

We call this kind of file to be **LibSVM format file** in this document, as all of the machine learning frameworks mentioned above call this file format to be **LibSVM format**.

### LibSVM Format Column

A **LibSVM format column** is a DBMS table column, whose value of each row is in the same format as each line in a **LibSVM format file**.

For example, in the following table, `c1` is a LibSVM format column, while `c2` is not a LibSVM format column.

```
+-------------------+------+
| c1                | c2   |
+-------------------+------+
| 1:2.1 3:4.5 5:6.0 |  2.8 |
| 7:-3.2 9:2.3      |  3.1 |
+-------------------+------+
```

## Background

SQLFlow extends SQL syntax to support the end-to-end machine learning pipeline. However, due to the variety of data formats, data pre-processing is usually a tough task for users. SQLFlow should do some automatic data pre-processing to make the end-to-end machine learning pipeline easier.

Currently, SQLFlow has supported loading the column data whose value of each row is:

- a number, like `10`, `-0.5`, etc.
- a string, like `"apple"`, etc.
- a number list encoded in the CSV format, like `"3,5,7"`, etc. The number list encoded in the CSV format may represent a dense or sparse vector.
    - If `"3,5,7"` represents a dense vector, it is `[3, 5, 7]`.
    - If `"3,5,7"` represents a sparse vector, each value of the sparse tensor `x` would only be 0 or 1, and `x[3] = x[5] = x[7] = 1`. 

We should support loading data from the LibSVM format column in SQLFlow, because:

- The LibSVM format column is a common way to store the sparse data in DBMS.
- There is [an issue](https://github.com/sql-machine-learning/sqlflow/issues/2323) that requires SQLFlow should support loading data from the LibSVM format column.

There would be 2 ways to load data from the LibSVM format column in SQLFlow:

- `TO RUN` statements. It can support any user-defined data pre-processing. We can release a docker image to convert the data from the LibSVM format column to a dense tensor, create columns for each element of the dense tensor, and write the data into the result table. However, if the data is highly sparse, it would create too many columns in the result table, and consume lots of time and memories to do the transformation.
- `COLUMN` clauses. The `COLUMN` clauses in SQLFlow support some commonly used feature columns, like `DENSE`, `SPARSE`, `CATEGORY_ID`, `EMBEDDING`, etc. If we want to support loading the data from the LibSVM format column, where would be 2 optional methods:
    - Add a new `COLUMN` clause, like `LIBSVM` or `SPARSE_KV`. It introduces complexity and disobeys the principle of Occam's Razor.
    - Extend the `SPARSE` column clause to support columns in different data formats. Currently, we have supported loading the number list encoded in the CSV format implicitly by the `SPARSE` column clause: in feature derivation stage, we would infer whether the column data is in the CSV format (see [here](https://github.com/sql-machine-learning/sqlflow/blob/3b70a0599beef573cd99f15dd41cc0a194634b75/pkg/ir/derivation.go#L146)), and convert the values in the CSV format into tensors in Python side (see [here](https://github.com/sql-machine-learning/sqlflow/blob/develop/python/sqlflow_submitter/db.py#L159)). We can do the same implicit feature derivation and data conversion for the LibSVM format column.
    
In conclusion, we would choose to extend the `SPARSE` column clause to support the LibSVM format column in this design.

## Only Support LibSVM Format Column Without Label Value

There are 2 kinds of LibSVM format column: with label value (e.g. `1 0:1.2 2:3.4`) and without label value (e.g. `0:1.2 2:3.4`). In this design, we only support the LibSVM format column without label value. It is because:

- The actual label value used for training would be confused if the LibSVM format column contains label value.
    - Users may choose the label in the LibSVM format column as the training label, but we have not supported to choose partial data of a column as the training label yet.
    - Users may choose another column as the training label, which is different from the label value in the LibSVM format column.
- If the LibSVM format column contains label value, users can easily split the label values into a separate column. For example, the SQL statement in MySQL is like:
    ```sql
    CREATE TABLE split_table AS 
    SELECT 
    SUBSTRING_INDEX(libsvm_column_with_label, ' ', 1) label, 
    TRIM(SUBSTR(libsvm_column_with_label, LOCATE(' ', libsvm_column_with_label))) libsvm_column_without_label
    FROM source_table;
    ```
    
Therefore, we choose to only support the LibSVM format column without label value in this design. If the LibSVM format column contains label value, users can split the label values to a separate column using the SQL statement above.

## Proposed Design

We would add a field named `format` in `FieldDesc`. It may be `CSV`, `LibSVM`, or other data format we would support in the future.

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

In the feature derivation stage, we can use a regular expression to check whether the column is a LibSVM format column. To avoid too much regular expression matching, this checking would be done only once when inferring the first row of the fetched samples in feature derivation stage. That is to say (in pseudo codes):

```go
func InferFeatureColumns() {
    rows := FetchSamples()
    rowCount := 0
    for rows.Next() {
        if rowCount == 0 {
            format := inferDataFormat(rows.Value()) // Use regular expression to infer the data format
            if format == "LibSVM" {
                // Fill FieldDesc info when the data format is LibSVM
                // For example: FieldDesc.Format = "LibSVM", FieldDesc.IsSparse = true, etc.
                ...
            }
        }
        rowCount ++
    }
}
```

After we have inferred the format of each column in the feature derivation stage, the data format information would be used in Python code generation. 

In the Python side, we would first read the raw data from the database, and then:

- For TensorFlow models: we would convert that data from the LibSVM format column into `tensorflow.SparseTensor` for training, prediction, evaluating, and explaining.
- For XGBoost models: we would dump the data from the LibSVM format column into LibSVM format files, which would be loaded as `xgboost.DMatrix` for training, prediction, evaluating, and explaining.

## SQL Statement Example

The SQL statement to read data from the LibSVM format column would be like:

```sql
SELECT * FROM train_table
TO TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN SPARSE(libsvm_column_name, 10000)
LABEL label
INTO result_table;
```

where users should write `SPARSE(column_name, shape)` to indicate that the column `column_name` stores sparse data. We would detect whether the `column_name` is a LibSVM format column in the feature derivation stage automatically.

The `shape` parameter in `SPARSE` is not required. If the `shape` is not provided, we would derive the dense shape of the data in the feature derivation stage.
