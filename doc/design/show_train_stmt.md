# Show Train Statement in SQLFlow

## Background

Like using other SQL engines, people may want to access the table metadata (say, the  table creating statement) using a SQL query. Usually they will type a statement like:
```
SHOW CREATE TABLE `test`;
```
which will produce:
```
mysql> SHOW CREATE TABLE `test`;
+-------+------------------------------------------------------------------+
| Table | Create Table                                                                                                                                                                                                                                                          |
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

It worth for SQLFlow to do the same thing. On SQLFlow platform, user may create a lot of models with a minor set of parameters different with each other. After a while, the  user may hardly tell what the differences are if he hadn't record the information else where (like in model name).  We would like to provide a formal way to solve this problem. Here we introduce the SHOW TRAIN feature.

## Grammar
```
SHOW TRAIN table_where_model_stored;
```
where
- `table_where_model_stored` must be a table which stores the trained model in MySQL or Hive

This query will show the original train sql statement like:
```
+-------+------------------------------------------------------------------+
| Table | Train Statement
+-------+------------------------------------------------------------------+
| test  | SELECT * FROM train to TRAIN xgboost.gbtree with learning_rate=0.4,
 objective=multi:softmax, num_class=3 LABEL class INTO test;
+-------+-----------------------------------------------------------------+
```
In the future we may support to show more metadata other than the original sql statement.

## Implementation

- Extend the SQLFlow parser with our `SHOW TRAIN` statement
- Branch to execute a `ShowTrain` cmd in executor, this cmd will first read the model on a `sqlfs` and  then extract the original train statement
- Format and route the cmd's output to the given output buffer

## Discuss
I noticed model zoo has created a table to store model meta in MySQL, shall we just use this mechanism to get the meta or extract from the saved model? How about the hive ones?
