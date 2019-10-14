sqlflow> SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, bucket(petal_length, [0., 1., 2.]), norm(petal_width)
LABEL class
INTO sqlflow_models.my_dnn_model;

