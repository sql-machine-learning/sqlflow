SELECT * FROM creditcardfraud
TRAIN DNNClassifier
COLUMN time,v1,v2...,v28,amount
LABEL class
INTO my_dnn_model;