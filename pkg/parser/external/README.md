# Integrating with Parsers of SQL Engines

## The Challenge

Typical SQLFlow statements are like the following.

```sql
SELECT * FROM training_data TO TRAIN DNNClassifier LABEL kind INTO my_model;

SELECT * FROM testing_data TO PREDICT testing_data.predicted_kind USING my_model;
```

The point is, assuming someone already has a SELECT statement for data cleaning and augmentation, (s)he could add a TO TRAIN or TO PREDICT clause to enable AI, no matter how complex the statement is or if it is hundreds of lines of code including nested SELECT.

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

    // The left part is a SELECT and the right part is TO TRAIN or TO PREDICT.
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

The above example program calls `sql_engine.Parse` twice before calling `SQLFlow.Parse`.  In practice, because Calcite parser is in Java, SQLFlow is a Go program, to enable SQLFlow calling Calcite parser, we have to wrap Calcite parser up into a command line tool which outputs the parsing result in JSON format.  It is time-consuming to make a Java command line call, so we pack the two calls to `sql_engine.Parse` into one call. Each call will output and result in JSON format and SQLFlow can parse the output into the following Go struct.

```
type ParseResult struct {
	SQL      []string `json:"sql"`
	Position int      `json:"position"`
	Error    string   `json:"error"`
}
```

Some external parsers are in Go and don't need command line call. For example, TiDB parser is in Go, and Hive parser is a set of AntLR grammar rules, and AntLR can generate Go code.  However, we would unify the interface to external parsers:

```go
func external_parser(kind, sql string) ([]string, int, error) {
   switch(kind) {
   case "mysql":
       return tidb.Parse(sql)
   case "calcite":
       return calcite.Parse(sql)
   ...
   }
}
```

Now we can change the above example program to call `external_parser` and `SQLFlow.Parse`.

```go
func Parse(sql_program string) (nodes, error) {
    allNodes := make([]nodes, 0)
    while len(sql_program) > 0 {
        // Start parsing by the third party parser
        nodes, err := parser.Parse(sql_program)
        if err != nil {
            // Error message from a parser should contain error position.
            pos := parseErrorPosition(err)
            leftPart = sql[:pos]
            rightPart = sql[pos:]

            nodes, errLeft := parser.Parse(leftPart)
            // In this case, the SQL is not acceptable due to the syntax error
            if errLeft != nil {
               return nil, err
            }

            // If leftPart is acceptable, it is a legitimate  SELECT statement.
            // We then try right part with SQLFlow parser using the extended syntax parser.
            node, errRight := esp.Parse(rightPart)
            if errRight != nil {
                return false, err
            }

            // Combine the select statement and the ML clause
            nodes[-1] = combineNode(nodes[-1], node)
        }
        allNodes = append(allNodes, nodes)
        sql_program = update(sql_program, nodes)

    return allNodes, nil
}
```
