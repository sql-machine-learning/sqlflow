# Support Arbitrary Select Statements

SQLFlow extends the SQL syntax to enable model training and inference. The syntax extension should be easy to learn and integrate well with the existing SQL syntax. While the standard SQL syntax supports complicated select statements, SQLFlow should support training/predict on these statements via appending a TRAIN/PREDICT clause right after them.

With the support of arbitrary select statements, a user can integrate AI into his/her database with ease.

## Overview

Given a long and nested select inside an extended SQL statement like

```sql
-- select clause
SELECT c1, c2, id, class FROM (SELECT * FROM my_table) a
-- train clause
TRAIN DNNClassifier
WITH n_class = 3
COLUMN c1, c2, EMBEDDING(id)
LABEL class
INTO my_dnn_model;
```

SQLFlow does the following steps.

1. SQLFlow splits the extended SQL statement into its select clause and its train clause. In the above example, the select clause is `SELECT ... a` and the train clause is `TRAIN DNNClassifier ... INTO my_dnn_model`. For the train clause, we check the syntax by our parser at `pkg/sql/parser.go`.

1. SQLFlow verifies the column in the train clause.
    1. SQLFlow executes the select clause and retrieves the column names and column types of the result. For example, the result of `SELECT ... a` has four columns with names `c1`, `c2`, `id`, and `class`. `c1`, and `c2` are of float types. And `id` and `class` are of integer types.
    1. SQLFlow checks the columns of train clause exist in the select result. For example, `c1`, `c2`, `id`, and `class` in `COLUMN` and `LABEL` are all in the select result. (Please be aware that `select expression` without an alias might give system-generated names that the user doesn't know in advance. For example, `select log(a + a) from my_table` may give a column named `log(a + a)` or any system-generated names. In this case, we suggested using alias such as `select log(a + a) as my_column_name`.)
    1. SQLFlow checks the columns have the desired types. The type is either suitable to explicit feature column transformation such as `EMBEDDING(id)`, or derived from [feature derivation](feature_derivation.md) such as `c1` of float type will be derived as a numerical column.

1. SQLFlow generates a Python submitter program that forwards the select clause to a particular SQL engine.

Please be aware that the SQLFlow parser does not parse the nested select due to the difficulty in handling different syntax across different SQL engines. Instead, it follows the UNIX's pipeline philosophy: forwarding the complexity to various SQL engines, while retrieves the data via unified database API.

## Implementation

### Splitting the Extended SQL

SQLFlow splits the extended SQL by looking for consecutive tokens returned by the lexer. If SQLFlow finds the consecutive tokens like [`TRAIN`, `IDENT`, `WITH`], [`PREDICT`, `IDENT`, `WITH`] or [`ANALYZE` `IDENT`, `WITH`], it splits the SQL string at the beginning of the first token in the list. For example, the lexer can go through the following SQL statement and find `TRAIN DNNClassifier WITH` satisfies the splitting criteria.

```sql
SELECT c1, c2, id, class FROM (SELECT * FROM my_table) a
-- splits at here
TRAIN DNNClassifier
WITH n_class = 3
COLUMN c1, c2, EMBEDDING(id)
LABEL class
INTO my_dnn_model;
```

### Verifier

The train clause requires column names returned by the select clause to exist and of certain types. To verifies these requirements, SQLFlow fetches column names and column types via executing the following template.

```SQL
SELECT a.* FROM ({.SelectClause}) AS a LIMIT 1;
```

### Create Temporary Training Table

`codegen.go` supports input being any SQL statements. So SQLFlow fills it with the select clause.

`codegen_alps.go` requires the input to be a table. So SQLFlow runs the following template to create a temporary table.

```SQL
CREATE TABLE {.TempTable} AS ({.SelectClause});
```
