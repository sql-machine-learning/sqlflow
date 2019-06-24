SELECT * FROM creditcardfraud
TRAIN sqlflow_models.MyDNNClassifier
COLUMN time,v1,v2...,v28,amount
LABEL class
INTO my_dnn_model;