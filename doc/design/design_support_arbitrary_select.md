# Support Arbitrary Select Statements

SQLFlow extends the SQL syntax to enable model training and inference. This extension should be easy to learn and integrate well with the existing SQL syntax such as nested SELECT statements. By appending a TRAIN or PREDICT clause after any select statement, we can add AI functionalities to our database with ease.

## Overview

Given a long and nested select inside the extended SQL statement like the following

```sql
SELECT c1, c2, c3, label              -- select clause
FROM table1 JOIN table2
ON table1.id = table2.id
WHERE condition
TO TRAIN DNNClassifier                -- train clause
WITH n_class = 3
COLUMN c1, c2, c3
OUTPUT label
INTO my_dnn_model;
```

SQLFlow does the following.

1. Splits the extended SQL statement into its select clause and the train clause. In the above example, the select clause is `SELECT ... WHERE condition` and the train clause is `TO TRAIN DNNClassifier ... INTO my_dnn_model`.
    1. For select clause, we check the syntax by pass it to a particular SQL engine. (MySQL/Hive/ODPS)
    1. For train clause, we check the syntax by our parser at `pkg/sql/parser.go`.

1. Verifies the extended SQL statement.
    1. For select clause, we verify that it is executable. And we also verify the column selected is either mentioned explicitly in the train clause or inferred by feature derivation.
    1. For train clause, we verify that the column names exists and have the desired types.

1. Generates a Python submitter program which forwards the select clause to a particular SQL engine to fetch the training data.

Please be aware that the SQLFlow parser does not parse the nested select due to the difficulty in handling different syntax across different SQL engines. Instead, it follows the UNIX's pipeline philosophy: forwarding the complexity to various SQL engines, while retrieves the data via unified database API.

![](/doc/figures/arbitrary-select.png)

## Implementation

### Splitting the Extended SQL

SQLFlow splits the extended SQL by looking for consecutive tokens returned by the lexer. If SQLFlow finds the following consecutive tokens [`TO`, `TRAIN`, `IDENT`], [`TO`, `PREDICT`, `IDENT`] or [`TO`, `EXPLAIN`, `IDENT`], it splits the SQL string at the beginning of the first token in the list.

### Syntactic Checking on the Select Clause

SQLFlow checks the syntax on the select clause by calling the particular SQL engine parser.

1. MySQL: SQL parser by PingCap https://github.com/pingcap/parser
1. Hive: Calcite parser https://calcite.apache.org/
1. MaxCompute: TBD

### Verifier

The train clause requires column names returned by the select clause to exist and of certain types. To verifies these
requirements, SQLFlow fetches column names and column types via executing the following template.

```SQL
SELECT a.* FROM ({.SelectClause}) AS a LIMIT 1;
```

### Code Generation

`codegen.go`'s supports all types of queries. So SQLFlow fills it with the select clause.

`codegen_alps.go` requires the input to be a table. So SQLFlow runs the following template to create a temporary table.

```SQL
CREATE TABLE {.TempTable} AS ({.SelectClause});
```
