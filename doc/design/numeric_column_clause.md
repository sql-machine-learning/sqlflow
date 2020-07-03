# Redesign the Numeric Column Clause in SQLFlow

## Definitions

### Numeric Column

A **numeric column** is a kind of DBMS table columns, whose cell value is:

- a single number. It includes an integer, a floating-point number, or a string that can be directly converted to an integer or floating-point number. For example, `1`, `3.5` or `"-102"`.
- a string that is encoded from a numeric vector. For example:
    - the string `"1.2,3.4,5.6"` is encoded from the numeric dense vector `[1.2, 3.4, 5.6]`.
    - the string `"1:1.2 3:3.4 5:5.6"` is encoded from the numeric sparse vector `[0, 1.2, 0, 3.4, 0, 5.6]` (if the length is 6). 

### Numeric Column Clause

A **numeric column clause** is a kind of SQLFlow `COLUMN` clauses, which is used to load the data from a **numeric column**. For example,

```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
COLUMN NUMERIC(c1), NUMERIC(SPARSE(c2)) -- This line is the numeric column clause
LABEL label
INTO result_table;
```

In this document, we would discuss why and how we should redesign the numeric column clause in SQLFlow.


## Background: Why Redesign the Numeric Column Clause

Currently, SQLFlow uses the following methods to support loading data from a numeric column:

- If each cell value is a single number, there are 3 equivalent ways to write the `TO TRAIN` SQL statement:
    - Do not write the `COLUMN` clause.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        LABEL label
        INTO result_table;
        ```
    - Write the column name directly in the `COLUMN` clause.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        COLUMN c1, c2
        LABEL label
        INTO result_table;
        ```
    - Write `NUMERIC()` or `NUMERIC(DENSE())` in the `COLUMN` clause.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        COLUMN NUMERIC(c1), NUMERIC(DENSE(c2))
        LABEL label
        INTO result_table;
        ```
        
- If each cell value is a string that is encoded from a numeric vector, the decoded vector of the cell value may be dense or sparse. For example, the string `"1,3,5"` may represent (1) a dense vector `[1, 3, 5]`, or (2) a sparse vector `x` , in which `x[1] = x[3] = x[5] = 1` and the other values of `x` are all zeros. Users should specify whether the decoded vector is dense or sparse explicitly in the `COLUMN` clause.
    - If the decoded vector is dense, there are 2 equivalent ways to write the `TO TRAIN` SQL statement:
        - Use the `NUMERIC()` in the `COLUMN` clause and specify the length of the decoded vector in `NUMERIC()`.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        LABEL label
        COLUMN NUMERIC(c1, 10), NUMERIC(c2, 10) -- 10 is the length of the decoded dense vector
        INTO result_table;
        ```
        - Use the `NUMERIC(DENSE())` in the `COLUMN` clause and specify the length of the decoded vector in `DENSE()`.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        LABEL label
        COLUMN NUMERIC(DENSE(c1, 10)), NUMERIC(DENSE(c2, 10)) -- 10 is the length of the decoded dense vector
        INTO result_table;
        ```
    - If the decoded vector is sparse, there is only one way to write the `TO TRAIN` SQL statement.
        ```sql
        SELECT c1, c2, label FROM train_table
        TO TRAIN DNNRegressor
        LABEL label
        COLUMN NUMERIC(SPARSE(c1, 10)), NUMERIC(SPARSE(c2, 10)) -- 10 is the dense length of the decoded sparse vector
        INTO result_table;
        ```

There are some problems in the current design:

- Both `NUMERIC` and `DENSE/SPARSE` have the `length/shape` parameter. 
For example, the expression `COLUMN NUMERIC(DENSE(c1, 10), 20)` is confused to users. 
Users do not know whether the length of `c1` is 10 (the `length/shape` parameter of `DENSE`) or 20 (the `length/shape` parameter of `NUMERIC`).
- `NUMERIC` is unnecessary in the `COLUMN` clause.
    - For the column whose cell value is a single number, we can write `COLUMN c1`, `COLUMN NUMERIC(c1)` or `COLUMN NUMERIC(DENSE(c1))`, and `COLUMN c1` is simpler.
    - For the column whose cell value is a string that is encoded from a numeric vector:
        - For dense data, we can write simpler expression `COLUMN DENSE(c1, length)` instead of `COLUMN NUMERIC(DENSE(c1, length))` .
        - For sparse data, we can write simpler expression `COLUMN SPARSE(c1, length)` instead of `COLUMN NUMERIC(SPARSE(c1, length))` .
        
In conclusion, we do not need the `NUMERIC` in the `COLUMN` clause. 
We can load the data from the numeric column just using `DENSE` and `SPARSE` in the `COLUMN` clause.

## Design

### Changes on the APIs

In the new design, we would remove `NUMERIC` in the `COLUMN` clause. We can use `DENSE/SPARSE` directly in the SQL statements.

For the column whose cell value is a single number, there would be still 3 equivalent ways to write the `TO TRAIN` SQL statement.
```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
LABEL label
INTO result_table;
```
or
```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
COLUMN c1, c2
LABEL label
INTO result_table;
```
or
```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
COLUMN DENSE(c1), DENSE(c2)
LABEL label
INTO result_table;
```

For the column whose cell value is a string that is encoded from a numeric vector, there would be only one way to write the `TO TRAIN` SQL statement.
- For dense data, the SQL statement would be:
```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
LABEL label
COLUMN DENSE(c1, 10), DENSE(c2, 10)
INTO result_table;
```
- For sparse data, the SQL statement would be:
```sql
SELECT c1, c2, label FROM train_table
TO TRAIN DNNRegressor
LABEL label
COLUMN SPARSE(c1, 10), SPARSE(c2, 10)
INTO result_table;
```

In the previous design, `NUMERIC` can be used with other `COLUMN` clauses together in SQL statements, like `BUCKET(NUMERIC())`, `EMBEDDING(NUMERIC())`, etc. In the new design, the `NUMERIC` in these SQL statements can be replaced with the `DENSE` or `SPARSE`.

- Example 1: we want to load the dense data from a numeric column and then transform the data using a `BUCKET` clause. The previous SQL statement would be:
    ```sql
    SELECT c1, c2, label FROM train_table
    TO TRAIN DNNRegressor
    LABEL label
    COLUMN BUCKET(NUMERIC(c1, 10), 100)
    INTO result_table;
    ```
    
    In the new design, the SQL statement would be (just replace `NUMERIC` with `DENSE`):
    ```sql
    SELECT c1, c2, label FROM train_table
    TO TRAIN DNNRegressor
    LABEL label
    COLUMN BUCKET(DENSE(c1, 10), 100)
    INTO result_table;
    ```

- Example 2: we want to load the sparse data from a numeric column and then transform the data using an `EMBEDDING` clause. The previous SQL statement would be:
    ```sql
    SELECT c1, c2, label FROM train_table
    TO TRAIN DNNRegressor
    LABEL label
    COLUMN EMBEDDING(NUMERIC(SPARSE(c1, 10)), 128)
    INTO result_table;
    ```

    In the new design, the SQL statement would be (just remove `NUMERIC`):
    ```sql
    SELECT c1, c2, label FROM train_table
    TO TRAIN DNNRegressor
    LABEL label
    COLUMN EMBEDDING(SPARSE(c1, 10), 128)
    INTO result_table;
    ```

### Changes on the Implementation

Although we would remove `NUMERIC` in the APIs, we can still unify both the `DENSE` and `SPARSE` feature columns as [`NumericColumn`](https://github.com/sql-machine-learning/sqlflow/blob/b9986f20eb0201845fb673684f885abe361aca02/pkg/ir/feature_column.go#L56) in Go side, because both of them are numeric features. 
Moreover, we can still distinguish whether the cell value of the column is dense or sparse by the [`FieldDesc.IsSparse`](https://github.com/sql-machine-learning/sqlflow/blob/b9986f20eb0201845fb673684f885abe361aca02/pkg/ir/feature_column.go#L36):

```go
type FieldDesc struct {
   Name      string
   DType     int
   Delimiter string
   Shape     []int 
   IsSparse  bool // indicates whether the cell value of the column is dense or sparse
   Vocabulary map[string]string
   MaxID int64
}
```

What we need to do is to rewrite the parser codes to parse `DENSE` and `SPARSE`, and SQLFlow should support some commonly used dense/sparse data formats. We would discuss which data formats should be supported in SQLFlow in future designs.
