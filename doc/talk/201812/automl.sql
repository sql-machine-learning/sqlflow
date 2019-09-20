sqlflow> SELECT *
FROM iris.train
TRAIN Classifier
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;

