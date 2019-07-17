# Design: Support arbitrary select statements

SQLFlow extends the SQL syntax to enable model training and inference. The SQL extension should be natural to learn and
integrate well with existing SQL programs despite many existing ones contain nested SELECT statements. Our SQLFlow
users could append a TRAIN or PREDICT clause in those programs and add AI functionalities with ease.

## Overview

Given a long and nested select inside the extended SQL statement, SQLFlow

1. splits the extended SQL statement into its select clause and the train clause.
    1. SQL syntactically check the select clause via a particular SQL engine parser. (MySQL/Calcite/ODPS)
    1. SQL The train clause will be syntactically checked by SQLFlow's parser.
1. verifies all fields used in train clause are produced by select clause and have the desired types.
1. generates Python submitter program which forwards the select clause to a particular SQL engine to fetch the training data.

Please be aware that SQLFlow doesn't attempt to parse the nested select due to the difficulty in handling different
syntax across SQL engines.

## Implementation

### Modification on SQLFlow parser

The SQLFlow parser is only responsible for parsing the train clause, instead of the full extended SQL statement. So the
[`standardSelect`](https://github.com/sql-machine-learning/sqlflow/blob/158b098cfecf7b12479171b09c12b877ad3fb00b/sql/sql.y#L66-L71)
struct will be a string.

### Splitting the extended SQL

SQLFlow splits the extended SQL by looking for consecutive tokens returned by the lexer. If SQLFlow finds the following consecutive tokens 
[`TRAIN`, `IDENT`, `WITH`] or [`PREDICT`, `IDENT`, `USING`], it splits the SQL string at the beginning of the first token
in the list.

### Syntactic checking on the select clause

SQLFlow checks the syntax on the select clause by calling the particular SQL engine parser.

1. MySQL: SQL parser by PingCap https://github.com/pingcap/parser
1. Hive: Calcite parser https://calcite.apache.org/
1. MaxCompute: TBD

### Verifier

The column clause requires fields returned by the select clause to exist and of certain types. The SQLFlow verifies these
requirements by fetching field names and field types via executing the following template.

```SQL
SELECT a.* FROM ({.SelectClause}) AS a LIMIT 1;
```

### Codegen

`codegen.go`'s supports all types of queries. So SQLFlow fills it with the select clause.

`codegen_alps.go` requires the input to be a table. So SQLFlow runs the following template to create a temporary table.

```SQL
CREATE TABLE {.TempTable} AS ({.SelectClause});
```
