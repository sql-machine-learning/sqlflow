# Show Train Statement in SQLFlow

## Background
Like using other SQL engines, people may want to access the table metadata (say, the `CREATE TABLE ` statement) using a SQL query. Usually they will type a statement like:
```
SHOW CREATE TABLE `test`;
```
which will produce:
```
mysql> SHOW CREATE TABLE `test`;
+-------+------------------------------------------------------------------+
| Table | Create Table
+-------+------------------------------------------------------------------+
| test  | CREATE TABLE `test` (
  `sepal_length` float DEFAULT NULL,
  `sepal_width` float DEFAULT NULL,
  `petal_length` float DEFAULT NULL,
  `petal_width` float DEFAULT NULL,
  `class` int DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci |
+-------+-----------------------------------------------------------------+
```

It worth for SQLFlow to do the same thing. On SQLFlow platform, user may create a lot of models with a minor set of parameters different with each other. After a while, the user may hardly tell what the differences are if he hadn't record the information else where (like in model name). We would like to provide a formal way to solve this problem. Here we introduce the `SHOW TRAIN` feature.

## Syntax
```
SHOW TRAIN table_where_model_stored ;
```
where
- `table_where_model_stored` must be a table which stores the trained model in MySQL or Hive. It should be a qualified table name like `{db_name}.{table_name}` if you are not in a DB selected context.

This query will show the original train sql statement like:
```
+-------+------------------------------------------------------------------+
| TABLE | TRAIN STATEMENT
+-------+------------------------------------------------------------------+
| test  | SELECT * FROM train to TRAIN xgboost.gbtree with learning_rate=0.4,
 objective=multi:softmax, num_class=3 LABEL class INTO test;
+-------+------------------------------------------------------------------+
```
In the future, we may support to show more metadata other than the original training sql statement.

## Implementation
- Extend the SQLFlow parser with our `SHOW TRAIN` statement. First, we need to add a key word `SHOW` to our extended syntax. In addition, `SHOW TRAIN` is not like our train/predict/explain statements which all share a `SELECT ... TO ...` format in which there is a **standard** `SELECT ...` part at the front and an **extended** `TO ...` part at the end. With this definition, our extending statement has no standard part. So, we have to modify the parse process slightly. The pseudo code is like below:
```go
func Parse(program string) ([]*SQLFlowStmt, error) {
  all := make([]*SQLFlowStmt)
  for program != "" {
    standard, err := tryExtractStandardPart(program)
    if err == nil {
      program = eatConsumedPrefix(program)
      if program == "" {
        // standard sql only
        all = append(all, standard)
      } else {
        // extract and verify the extended 'TO ...' part
        extended, err: = tryParseExtendedSyntax(program)
        if err != nil {
          return nil, err
        }
        if isValidToExtended(merged)
          merged := merge(standard, extended)
          all = append(all, merged)
        } else {
          return nil, err
        }
      }
    } else if extended, e := tryParseExtendedSyntax(program); e == nil {
      // Maybe it is a pure extended stmt with no standard part (say, `SHOW TRAIN...`), so try to use the extended parser
      // In addition, the program should not contains a 'TO...' only, although our extended parser accepts this form
      if isValidPureExtended(extended) {
        all = append(all, extended)
      } else {
        return nil, err
      }
    } else {
      return nil, err
    }
    program = eatConsumedPrefix(program)
  }
  return all, nil
}
```
- Branch to execute a `ShowTrain` cmd in executor, this cmd will first read the model on a `sqlfs` and then extract the original train statement. We can not get the train statement from models saved on OSS because they have not been saved with the models yet. This can be implemented next time.
- Format and route the cmd's output to the given output buffer

## Discussion
I noticed model zoo has created a table to store model meta in MySQL, shall we just use this mechanism to get the meta or extract from the saved model?
- No, this table will be deleted in the future
