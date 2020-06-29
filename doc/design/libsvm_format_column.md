# Support LibSVM Format Column in SQLFlow

## Definition

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
- the label corresponding to the sample is 0.

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

Currently, SQLFlow only supports loading the column value which is a number, a string, or a number list in CSV format. We should support loading data from the LibSVM format column in SQLFlow, because the LibSVM format column is a common way to store the sparse data in DBMS, and there have been [some issues](https://github.com/sql-machine-learning/sqlflow/issues/2323) that require SQLFlow should support loading data from the LibSVM format column.

There are 2 ways to support the LibSVM format column in SQLFlow:

- `TO RUN` statements. It can support any user-defined data pre-processing. We can release a docker image to convert the data from the LibSVM format column to a dense tensor, create columns for each element of the dense tensor, and write the data into the result table. However, if the data is highly sparse, it would create too many columns in the result table, and consume lots of time and memories to do the transformation.
- `COLUMN` clauses. The `COLUMN` clauses in SQLFlow support some commonly used feature columns, like `NUMERIC`, `CATEGORY_ID`, `EMBEDDING`, etc. If we want to support loading the data from the LibSVM format column, where would be 2 optional methods:
    - Add a new `COLUMN` clause, like `LIBSVM` or `NUMERIC_KV`. It introduces complexity and disobeys the Occam's Razor principle.
    - Extend the `NUMERIC` column clause to support columns in different data formats. Currently, we have supported CSV format implicitly in `NUMERIC` column clause: in feature derivation stage, we would infer whether the column data is in CSV format (see [here](https://github.com/sql-machine-learning/sqlflow/blob/3b70a0599beef573cd99f15dd41cc0a194634b75/pkg/ir/derivation.go#L146)), and convert the values in CSV format into tensors in Python side (see [here](https://github.com/sql-machine-learning/sqlflow/blob/develop/python/sqlflow_submitter/db.py#L159)). We can do the same implicit feature derivation and conversion for the LibSVM format column.
    
In conclusion, we would choose to extend the `NUMERIC` column to support the LibSVM format column in this design.

## Only Support LibSVM Format Column Without Label Value

There are 2 kinds of LibSVM format column: with label value (e.g. `1 0:1.2 2:3.4`) and without label value (e.g. `0:1.2 2:3.4`). In this design, we only support the LibSVM format column without label value. It is because:

- The actual label value used for training would be confused if the LibSVM format column contains label value.
    - Users may choose the label in the LibSVM format column as the training label, but we have not supported to choose partial data of a column as training label yet.
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

## Design

### Add Format Field in FieldDesc

Add a field named `format` in `FieldDesc`. It may be `CSV`, `LibSVM`, or other data format we would support in the future.

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

### Implicit Data Format Derivation in Feature Derivation Stage

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

### Convert the Values of the LibSVM Format Column to Sparse Tensor in Python Side

After we have inferred the format of each column in the feature derivation stage, and the data format information would be used in Python code generation. In the Python side, we would first read the raw data from the database, and then convert the data from the LibSVM format column into sparse tensors for training, prediction, evaluating, and explaining.

### SQL Statement Example

The SQL statement to read data from LibSVM format column would be like:

```sql
SELECT * FROM train_table
TO TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN NUMERIC(libsvm_column_name, 10000)
LABEL label
INTO result_table;
```

where `NUMERIC(column_name, shape)` column clause is optional. If users provide the `NUMERIC` column clause in the SQL statement, `shape` parameter in `NUMERIC` is required to indicate the dense shape of the sparse data; if not, we would derive the dense shape of the data in feature derivation stage.
