sqlflow> SELECT *
FROM iris.train
TO TRAIN Classifier
LABEL class
INTO sqlflow_models.my_dnn_model;

