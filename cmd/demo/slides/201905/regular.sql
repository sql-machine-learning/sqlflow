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

