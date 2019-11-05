sqlflow> SELECT * FROM employee LIMIT 3;
+------------+------------+-----------+-----------+
|    name    |   gender   |    age    |   score   |
+------------+------------+-----------+-----------+
|   "Tony"   |   "Male"   |     32    |    550    |
+------------+------------+-----------+-----------+
|   "Nancy"  |   "Female" |     29    |    660    |
+------------+------------+-----------+-----------+
|   "Well"   |   "Unkown" |     30    |    590    |
+------------+------------+-----------+-----------+

sqlflow> SELECT * FROM employee
TO TRAIN LogisticRegression
COLUMN categorical_column_with_hash_bucket(name, 10), categorical_column(gender), age
LABEL salary
INTO my_project.my_lr_model;
