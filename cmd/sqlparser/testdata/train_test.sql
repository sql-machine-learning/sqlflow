SELECT employee.age, last_name, salary
FROM   employee
LIMIT  100
WHERE
  employee.age % 10 < (salary / 10000)
  AND
  strings.Upper(last_name) = "WANG"
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(emplyoee.name), bucketize(last_name, 1000))
INTO
  my_dnn_model
;
