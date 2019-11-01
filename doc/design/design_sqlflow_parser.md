# SQLFlow Parser

SQLFlow's user experience should integrate smoothly with SQL programmer's daily work routine, which involves

1. Submitting multiple SQL statements at once.
1. Working with complicated select statements.

So the SQLFlow parser should

1. Support any SQL dialect.
1. Accept a SQL program contains multiple SQL statements.
1. Support arbitrary select statements for extended syntax.

The design of SQLFlow parser should support the above requirements.

## Overview

**The third-party parser** means the pre-existing parser of a particular SQL engine, like HiveQL/Calcite/TiDB parser. It supports parsing on a specific SQL dialect.

**The extended syntax parser** means the parser written by the SQLFlow team to parse the extended syntax like `TO TRAIN/PREDICT/EXPLAIN ...`.

**The SQLFlow parser** means the combination of the above two parsers. It supports parsing a SQL program of any SQL dialect.

Upon receiving a SQL program like the following (`SELECT ...` represents a complicated select statement).

```SQL
CREATE TABLE my_training_table AS SELECT ...;

SELECT ... TO TRAIN ...;

SELECT ... TO PREDICT ...;

SELECT * FROM predict_result_table;
```

The SQLFlow splits it into four statements.
- For standard SQL statements like the `CREATE ...` and `SELECT * FROM predict_result_table`, SQLFlow calls the third party parser to check syntax.
- For extended SQL statements like the second and the third statement, SQLFlow parser further splits the statement into two parts, namely a standard select statement and a train/predict statement.
    - SQLFlow verifies the first half is a syntactically correct select statement.
    - SQLFlow parses the second half using the extended parser.

## Design Choices

### Lexer vs. Third-Party Parser

One major technical challenge is splitting, which includes

1. Splitting a SQL program into multiple SQL statements.
1. Splitting an extended SQL statement into a standard select statement and a train/predict statement.

There are two possible solutions.

#### Solution 1: Splitting using Lexer

SQLFlow lexer scans the SQL program and receives a sequence of tokens.

SQLFlow splits the SQL program by the `;` token.

SQLFlow splits the extended SQL by looking for consecutive tokens returned by the lexer. If SQLFlow finds the consecutive tokens like [`TO`, `TRAIN`] or [`TO`, `PREDICT`], it splits the SQL string at the beginning of the first token in the list. For example, the lexer can go through the SQL statement `SELECT ... TO TRAIN` and find it satisfies the splitting criteria.

Pros:
1. Straight forward to implement.

Cons:
1. Incompleteness: single quote, double quote, square parentheses have different meanings in different SQL dialects. SQLFlow Lexer can't tokenize SQL programs of varying SQL dialects uniformly. This may lead to errors in the splitting.
1. Needs to deal with special cases like `ALTER TABLE table_name RENAME TO train`.

#### Solution 2: Splitting using the Third Party Parser

SQLFlow begins parsing by using the third party parser(TPP). If TPP encounters an error at position `p`, TPP redoes parsing on the SQL statement before `p`, if success, extended SQL parser will parse on the statement after `p`. SQLFlow repeat these steps until the end of the SQL program.

To demonstrate the above logic, let's consider the following SQL program.

```SQL
CREATE TABLE my_training_table AS SELECT ...;

SELECT ... TO TRAIN ...;

SELECT ... TO PREDICT ...;

SELECT * FROM predict_result_table;
```

SQLFlow does the following step.

1. TPP parses on the first line, stop at the `;`, and succeed.
1. TPP parses on the second line and encounters an error at `TO TRAIN`.
    1. TPP parses the `SELECT ...` and succeeds.
    1. The extended parser parses `TO TRAIN ...`, stops at `;` and succeeds.
1. TPP parses on the third line and encounters an error at `TO PREDICT`.
    1. TPP parses the `SELECT ...` and succeeds.
    1. The extended parser parses `TO PREDICT ...`, stops at `;` and succeeds.
1. TPP parse on the fourth line and succeed.

Pros:
1. We can support different SQL dialects.
1. We start parsing from the beginning, so the TPP has a chance to raise useful error messages based on its global view.
1. Distinguishing a SQL statement between standard SQL and extended SQL becomes trivial (i.e., if the third party parser can't parse the statement alone, then it is an extended SQL statement).
1. Splitting a SQL program into multiple SQL statements becomes trivial.

Cons:
1. The error position `p` raise by TPP might be inaccurate. However, we haven't found any counterexample in our experiments at #1046 and #1103.

Based on the above pros and cons, we choose the TPP solution.

### gRPC vs. Local Command Line

SQLFlow is written in Go, third party parsers for Calcite/HiveQL are written in Java. So we need to call the third parse via either gRPC or local command line.

We prefer the local command line for the following reason.
1. We can avoid setting up an additional gRPC server, which makes the testing/deployment simpler. And because there is no downstream call for the parser, wrapping the parser as a service might be overkill.
1. We can guarantee the isolation between parsing requests. The gPRC server may fail due to an unexpected request, meaning one request can affect the other.

## Implementation Details

### Change `TRAIN/PREDICT` to `TO TRAIN/PREDICT`

SQLFlow syntax should be compatible with the standard SQL syntax. During our earlier experiments, we found using `TRAIN` along to denote the SQL extension is not enough. In cases like
1. `select * from mytable train dnn ...`, the SQL syntax will treat `train` as an alias of `mytable`.
1. `select * from mytable train` where we eliminate the model name because of the automatic model selection.

These cases lead us to the motivation of changing `train` to `to train`. All [MySQL](https://dev.mysql.com/doc/refman/5.5/en/keywords.html), [MaxCompute](https://www.alibabacloud.com/help/doc-detail/27872.htm?spm=a2c63.p38356.879954.6.631c5095hrjubf#concept-zxk-v5f-vdb), and [Hive](https://cwiki.apache.org/confluence/display/Hive/LanguageManual+DDL) consider `TO` as a reversed keyword. And its usage is subjected in the following scenario.
1. Hive: `ALTER TABLE table_name RENAME TO new_table_name;`.
1. MySQL: `TO` is used in [`GRANT`](https://dev.mysql.com/doc/refman/8.0/en/grant.html) and [`RENAME`](https://dev.mysql.com/doc/refman/8.0/en/rename-table.html).
1. MaxCompute:
    1. [`ALTER TABLE table_name RENAME TO new_table_name;`](https://www.alibabacloud.com/help/doc-detail/73768.html?spm=a2c5t.11065259.1996646101.searchclickresult.5afd4bd7qECSMQ)
    1. [`alter table table_name changeowner to 'ALIYUN$xxx@aliyun.com';`](https://www.alibabacloud.com/help/doc-detail/73768.html?spm=a2c5t.11065259.1996646101.searchclickresult.5afd4bd7qECSMQ)

So Given a SQL statement `SELECT ... TO TRAIN`, the third-party parser should raise error precisely at the position of `TO`.

### Third Party Parser API

The third-party parser provides a function `ParseAndSplit`, it takes a SQL program and returns three elements: `statements`, `position`, and `error`.

```text
ParseAndSplit takes a SQL program.

It returns <statements, -1, ""> if the third party parser accepts the SQL program.
    input:  "select 1; select 1;"
    output: {"select 1;", "select 1;"}, -1 , nil
It returns <statements, idx, ""> if Calcite parser accepts part of the SQL program, indicated by idx.
    input:  "select 1; select 1 to train; select 1"
    output: {"select 1;", "select 1"}, 19, nil
It returns <nil, -1, error> if an error is occurred.
```

### Remove Comments

Some third-party parsers don't support parsing multiple SQL statements at one parse call. So at the end of each parse call, we need to check if the end position is a `;`. If so, and if there is a statement after, we should continue parsing. However, it is tricky to tell if there is a statement after. One possible solution is to remove all SQL comments and checked the trimmed version of the rest string; if the string length is greater than zero, we should continue parsing.
