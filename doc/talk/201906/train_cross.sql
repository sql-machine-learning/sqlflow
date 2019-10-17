SELECT * FROM survey 
TRAIN DNNRegressor
COLUMN *, cross(gender, age) 
LABEL income
INTO my_model;
