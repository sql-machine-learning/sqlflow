# SQLFlow Demo

Before you start, you need to set up test MySQL server as described [here](../../example/datasets)

Start SQLFlow as
```golang
go get -d ./... && go run main.go -logdir="/path/to/logs/" -loglevel="info"
```

#### Regular SQL Statements

```sql
sqlflow> SHOW DATABASES;
+--------------------+
| Database           |
+--------------------+
| churn              |
| information_schema |
| iris               |
| mysql              |
| performance_schema |
| sqlflow_models     |
| sqlfs              |
| sys                |
+--------------------+
sqlflow> SELECT * FROM iris.train LIMIT 1;
+--------------+-------------+--------------+-------------+-------+
| sepal_length | sepal_width | petal_length | petal_width | class |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
+--------------+-------------+--------------+-------------+-------+
```

#### Train

```sql
sqlflow> SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
-----------------------------
2018/12/16 15:03:54 tensorflowCmd: run in Docker container
Job success
```

#### Predict

```sql
sqlflow> SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;
-----------------------------
2018/12/16 15:05:58 tensorflowCmd: run in Docker container
Job success
```

The prediction is stored at `iris.predict`, you can look them up at

```sql
sqlflow> SELECT * FROM iris.predict LIMIT 10;
-----------------------------
+--------------+-------------+--------------+-------------+-------+
| sepal_length | sepal_width | petal_length | petal_width | class |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
|            5 |         2.3 |          3.3 |           1 |     1 |
|          4.9 |         2.5 |          4.5 |         1.7 |     2 |
|          4.9 |         3.1 |          1.5 |         0.1 |     0 |
|          5.7 |         3.8 |          1.7 |         0.3 |     0 |
|          4.4 |         3.2 |          1.3 |         0.2 |     0 |
|          5.4 |         3.4 |          1.5 |         0.4 |     0 |
|          6.9 |         3.1 |          5.1 |         2.3 |     2 |
|          6.7 |         3.1 |          4.4 |         1.4 |     1 |
|          5.1 |         3.7 |          1.5 |         0.4 |     0 |
+--------------+-------------+--------------+-------------+-------+
```
