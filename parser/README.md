# Integrating with Parsers of SQL Engines

## The Challenge

Typical SQLFlow statements are like the following.

```sql
SELECT * FROM training_data TRAIN DNNClassifier LABEL kind INTO my_model;

SELECT * FROM testing_data PREDICT testing_data.predicted_kind USING my_model;
```

The point is, assuming someone already has a SELECT statement for data cleaning and augmentation, (s)he could add a TRAIN or PREDICT clause to enable AI, no matter how complex the statement is or if it is hundreds of lines of code including nested SELECT.

A significant challenge for SQLFlow here is to parse the SQL. Many SQL engines, such as MySQL, Oracle, Hive, SparkSQL, Flink, claim they are compatible with ANSI SQL but most have unique features.

Think about the following facts that (1) each parser might be very complicated, for example, MySQL's parser file sql_yacc.yy has >16,000 lines of code, and (2) some dialects have contradictory uses of the same keyword, it is intractable to merge multiple dialect parsers in the "SQLFlow parser" and keeps it compatible with the development of the dialects.

An alternative is to re-implement SQLFlow for each SQL engine as their extensions or UDF (user-defined function). This approach is equally intractable as there are so many engines to support.

## Overall Design

We design SQLFlow as a wrapper of these SQL engines, which means that SQL statements come directly to SQLFlow and SQLFlow decides if it follows the dialect syntax of the specified SQL engine. If so, SQLFlow proxies the statement to the SQL engine, otherwise, SQLFlow translates it into a Python program, a.k.a., the submitter program, which calls the SQL engine and the AI engine to train or to predict.

With this design, the key challenge becomes to judge if a SQL statement is acceptable by a SQL engine's parser. We propose the following solution: 

```go
func Parse(sql string) (acceptable bool, err error) {
    // Where SQL is acceptable by the original engine, no error occurred.
    err := sql_engine.Parse(sql)
    if err == nil {
        return true, ni
    }
    
    // Error message from a parser should contain error position.
    pos := parseErrorPosition(err) 
    leftPart = sql[:pos]
    rightPart = sql[pos:]

    errLeft := sql_engine.Parse(leftPart)
    // In this case, the SQL is not acceptable due to the syntax error
    if err != nil {
       return false, err 
    }

    // If leftPart is acceptable, it is a legitimate  SELECT statement. We then try right part with SQLFlow parser. 
    errRight := SQLFlow.Parse(rightPart)
    if err != nil {
        return false, errRight 
    }

    // The left part is a SELECT and the right part is TRAIN or PREDICT.
    return false, nil 
}
```

This procedure has two assumptions: (1) a SQL engine has a parser, denoted by sql_engine.Parse, and (2) the parser reports the position of syntax errors if there is any.  Both assumptions are reasonable. However, a key question here is how to call parsers of various SQL engines. For open source SQL engines, this doesn't seem an issue. Here we list parsers used by some well-known engines:

| SQL Engine | Parser  |
|------------|---------|
| Beam       | Calcite |
| Flink      | Calcite |
| Storm      | Calcite |
| Hive       | Hplsql.g4 |
| MySQL      | sql_parse.{h,cc} |

It is notable that MySQL doesn't provide sufficient documentation on how to call its parser, so we call a Go implementation of MySQL's SQL parser by PingCap https://github.com/pingcap/parser instead. For proprietary engines, we have a plan to ask the licensing of their parsers.


## The External Parser Abstraction

The above example program calls `sql_engine.Parse` twice before calling `SQLFlow.Parse`.  In practice, because Calcite parser is in Java, SQLFlow is a Go program, to enable SQLFlow calling Calcite parser, we have to wrap Calcite parser up into a gRPC server.  It is time-consuming to make an RPC call, so we pack the two calls to `sql_engine.Parse` into one:

```protobuf
message ParserRequest {
  string query = 1;
}

message ParserResponse {
  string sql = 1;
  string extension = 2;
  string error = 3;
}

service Parser {
  rpc Parse (ParserRequest) returns (ParserResponse) {}
}
```

Some external parsers are in Go and don't need RPC.  For example, TiDB parser is in Go, and HiveQL parser is a set of AntLR grammar rules, and AntLR can generate Go code.  However, we would unify the interface to external parsers:

```go
func external_parser(kind, sql string) (idx int, err error) {
   switch(kind) {
   case 'calcite':
       r := grpc.Call("Parser.Parse", ParserRequest{sql})
       return r.GetIndex(), r.GetError()
   case 'mysql':
       return tidb.Parse(sql)
   ...
   }
}
```

Now we can change the above exmaple program to call `external_parser` and `SQLFlow.Parse`.

```go
func Parse(sql string) error {
    i, e := external_parser(sql)
    if e != nil {
      return e
    }
    
    if i != -1 {
      return SQLFlow.Parse(sql[i,:]) // Parse the right part.
    }
    return nil
}
```

The above design of `external_parser` returns `idx int`, which separate the input string `sql` into the left part, the "standard" SQL, and the right part, the SQLFlow extension.  In reality, we need it to return more than `idx`.


## Get Prepared for Logical Verification

The parser verifies the syntax of the input.  SQLFlow also needs to check the logic.  For example, have a look at the following SQLFlow statement

```sql
SELECT a, b FROM t1 TRAIN DNNClassifier COLUMN c LABEL b;
```

There is no syntax error in the above example; however, it has a logical mistake -- the column `c` is undefined.

In the design of SQLFlow, we do verification after parsing.  The verifier would like to know the field and table named mentioned in the statement, which is supposed to be returned by `external_parser` in addition to `idx`. Both TiDB parser and Calcite parser return an abstract syntax tree; it seems that we can traverse the tree and find out the field and table names.


## Directory Structure

The [TiDB parser](https://github.com/pingcap/parser) is in Go, so SQLFlow server can make local calls to it.  Some others like [Calcite parser](https://github.com/apache/calcite/tree/master/core/src/main/java/org/apache/calcite/sql/parser) and  [Hive parser](https://github.com/apache/hive/tree/master/ql/src/java/org/apache/hadoop/hive/ql/parse) are in Java, or some other languages, and we need remote calls like gRPC.  We refer the later kind by *remote parsers*.  All remote parsers must implement the same gRPC interface defined in `remote/paser.proto`.

```protobuf
service Parser {
  rpc Parse (Request) returns (Response) {}
}
```

Hence, the directory structure is like the following:

```
parser/
    tidb/
	    tidb_parser.go
	remote/
	    grpc/
		    src/main/proto/parser.proto
	    calcite/
		    src/main/java/org/sqlflow/parser/CalciteParserServer.java
		hiveql/
		    src/main/java/org/sqlflow/parser/HiveQLParserServer.java
	    client.go
```
