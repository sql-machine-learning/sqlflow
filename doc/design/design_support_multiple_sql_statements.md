# Support Multiple SQL Statements

## Overview

This is a design doc on supporting multiple SQL statements in one magic command cell.

## Problem

Currently, our Jupyter magic command `%%sqlflow` only supports to run one statement a time. If a user has multiple SQL statements to run, he/she needs to type `%%sqlflow` many times, which is not ideal.

## Design Choice

There are several design choices to make.

### Splitting Location: Client-Side vs Server-Side

While splitting at the client-side is relatively simple to implement. We prefer to split at the server-side for the following reasons.
1. Loose coupling. SQLFlow server can use its lexer/parser to split SQL statements accurately. 
1. Extensibility. SQLFlow server can run a sequence of SQL statements to form a workflow. In future work, we would like to analyze the dependencies of the execution order of all the SQL statements to create an execution graph automatically, then parallelly execute the graph nodes.

### Splitting Technique: Hybrid Parser vs. Lexer

The hybrid parser solution uses the third-party SQL parser (like [TiDB parser](https://github.com/pingcap/parser/blob/master/parser.y)) and SQLFlow parser to determine the end of an SQL statement. The third-party SQL parser first parses the extended SQL statement. It will raise error near SQLFlow extended keywords, like TRAIN and PREDICT. Then the SQLFlow parser starts from the error position and stops at the end of the first statement. However, this solution relies on the third-party SQL parser to report the error **accurately** on the keywords, like TRAIN and PREDICT, that it can't recognize.

The lexer solution scans the entire SQL statements, finds the `;` tokens, and splits the SQL based on the position of  `;` token.

We choose the lexer solution due to its sufficiency and simplicity.

## Implementation

We add an `EndOfExecution` message type to the gRPC protocol buffer definition to indicate the end of an SQL statement execution. So the client should able to distinguish responses message of different SQL statements.

In addition to the job message, the server should also send the currently running SQL statements so that the user can keep track of the progress. For example

```
%%sqlflow
select ... train ...;
select ... predict ...;
--------------------------------------------------
start running the first SQL: select ... train ...;

<log>
<log>

finished running the first SQL: select ... train ...;
total time: ... s

start running the second SQL: select ... predict ...;

<log>
<log>

finished running the second SQL: select ... predict ...;
total time: ... s
```

We implement the split function using the lexer in the `sql` package; then we expose this function as `SplitSQLStatements(s string) []string` for the `server` package. The server calls this function then feeds the result to `sql.Run` one by one.
