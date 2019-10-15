sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
COLUMN sepal_length, sepal_width, petal_length, petal_witdh
LABEL calss            -- error: cannot find field calss
INTO sqlflow_models.my_dnn_model;

sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
COLUMN sepal_length, sepal_width, petal_length
LABEL petal_witdh      -- error: DNNClassifer's label should of type INT, received FLOAT
INTO sqlflow_models.my_dnn_model;
