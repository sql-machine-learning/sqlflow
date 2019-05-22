sqlflow> SELECT *
FROM iris.train
TRAIN Classifier
LABEL class
INTO sqlflow_models.my_dnn_model;

