SELECT * FROM survey 
TO TRAIN DNNRegressor
COLUMN bucketize(hash(name), 100), categorize(gender), age 
LABEL income
INTO my_model;
