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
