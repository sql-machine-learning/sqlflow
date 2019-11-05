SELECT * FROM survey 
TO TRAIN DNNRegressor
COLUMN *, cross(gender, age) 
LABEL income
INTO my_model;
