# SQLFlow Parser

SQLFlow is a translator that translates a SQL program to a workflow program. The first step of this translation is parsing the SQL program, which is the focus of this design doc.

## Overview

A typical SQL program includes **standard SQL statements** and **extended SQL statements**.

The **standard SQL statements** is defined by the existing SQL engines like MySQL, Hive, Alibaba MaxCompute. Because different SQL engines have different syntax, we use the engine's parsers to parse the standard SQL statement. We denote this parser as the **third party parser(TPP)** in this design doc.

The **extended SQL statements** is defined by appending a train/predict/explain clause, or **ML clause** for short, right after a select statement. We use TPP to parse the select statement because we want its syntax to be consistent with the standard SQL. And we implement an **extended syntax parser(ESP)** to parse the ML clause.

Take the following SQL program for example, it has five SQL statements. The first, second, fifth statements are standard SQL statements, and the third and fourth are extended SQL statements. TPP is responsible for parsing `CREATE TABLE my_training_table ...`, `CREATE TABLE my_test_table ...`, `SELECT * FROM my_training_table`, `SELECT * FROM my_test_table`, and `SELECT * FROM my_prediction`. And ESP is responsible for parsing `TO TRAIN ...` and `TO PREDICT ...`.

```SQL
CREATE TABLE my_training_table AS SELECT employee WHERE onboard_year < 2018;

CREATE TABLE my_test_table AS SELECT employee WHERE onboard_year >= 2018;

SELECT * FROM my_training_table
TO TRAIN MyDNNRegressor
LABEL class
INTO my_model_table;

SELECT * FROM my_test_table
TO PREDICT my_predict_table
USING my_model_table;

SELECT * FROM my_predict_table;
```

After the parsing of TTP and ESP, SQLFlow will generate a workflow based on the parsed results.

## Design Choices

SQLFlow needs to decide where to call which parser. Here are two proposals.

In the first proposal, we use the lexer of the ESP to scan the SQL program and receive a sequence of tokens. SQLFlow splits the SQL program by the `;` token and gets a list of substrings. In each substring, SQLFlow splits the extended SQL by looking for consecutive tokens like [`TO`, `TRAIN`], or [`TO`, `PREDICT`]. If found, SQLFlow splits the substring at the beginning position of the `TO` token. TPP will parse the first half of the substring, and ESP will parse the second half. If not found, TPP will parse the whole substring. For example, the lexer can go through the SQL statement like `SELECT ... TO TRAIN` and find it satisfies the splitting criteria.

While this proposal is straight forward to implement, it is incomplete. Because ESP's lexer can't tokenize SQL programs uniformly across different SQL dialects, this proposal may lead to errors in the splitting. Also, SQLFlow needs to deal with special cases like `ALTER TABLE table_name RENAME TO train`, which is a standard SQL statement by will be considered as an extended SQL statement in the proposal.

In the second proposal, SQLFlow begins parsing by using the TPP. If TPP encounters an error at position `p`, TPP will try to parse on the SQL statement before `p`, if success, ESP will parse on the statement after `p`. SQLFlow repeat these steps until the end of the SQL program. We can put this logic as the following pseudo-code.

```go
func Parse(sql_program string) (nodes, error) {
    allNodes := make([]nodes, 0)
    while not done processing sql_program {
        // Start parsing by the third party parser
        nodes, err := tpp.Parse(sql_program)
        if err != nil {
            // Error message from a parser should contain error position.
            pos := parseErrorPosition(err) 
            leftPart = sql[:pos]
            rightPart = sql[pos:]

            nodes, errLeft := tpp.Parse(leftPart)
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
    }
    

    return allNodes, nil 
}
```

This proposal has many advantages:
1. We start parsing from the beginning, so the TPP has a chance to raise useful error messages based on its global view.
1. Distinguishing a SQL statement between standard SQL and extended SQL becomes trivial (i.e., if the TPP can't parse the statement alone, then it is an extended SQL statement).
1. Splitting a SQL program into multiple SQL statements becomes trivial.

One concern is that the error position `p` raise by TPP might be inaccurate. However, we haven't found any specific case in our experiments at #1046 and #1103.

So we choose the second proposal two over the first proposal.

## Implementation Details

### Change `TRAIN/PREDICT/EXPLAIN` to `TO TRAIN/TO PREDICT/TO EXPLAIN`

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

### Call the Third-Party Parser

SQLFlow is written in Go, third party parsers for Calcite/Hive are written in Java. So we need to call the third parse via either gRPC or local command line. We prefer the local command line for the following reasons. Firstly, we can avoid setting up an additional gRPC server, which makes the testing/deployment simpler. And because there is no downstream call for the parser, wrapping the parser as a service might be overkill. Secondly, We can guarantee the isolation between parsing requests. The gPRC server may fail due to an unexpected request, meaning one request can affect the other.

### Remove Comments

Some third-party parsers don't support parsing multiple SQL statements at one parse call. So at the end of each parse call, we need to check if the end position is a `;`. If so, and if there is a statement after, we should continue parsing. However, it is tricky to tell if there is a statement after. One possible solution is to remove all SQL comments and checked the trimmed version of the rest string; if the string length is greater than zero, we should continue parsing.
