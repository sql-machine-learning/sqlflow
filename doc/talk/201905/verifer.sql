sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
COLUMN sepal_length, sepal_width, petal_length, petal_witdh
LABEL calss
INTO sqlflow_models.my_dnn_model;
------------------------------------------------------
error: cannot find field calss


sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
COLUMN sepal_length, sepal_width, petal_length
LABEL petal_witdh
INTO sqlflow_models.my_dnn_model;
------------------------------------------------------
error: DNNClassifer's label should of type INT
