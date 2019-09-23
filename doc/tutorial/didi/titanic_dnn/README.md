# Classify Titanic Dataset Using DNNClassifer

This tutorial demonstrates how to
1. train a DNNClassifer model on Titanic dataset.
2. use the trained model to predict the class of the passenger's survival status.

## The Dataset

The Titanic datasets `train.csv` and `test.csv` in this directory come from [Kaggle](https://www.kaggle.com/c/titanic). The task is to predict which passengers survived the tragedy.

We have used feature engineering to preprocess the raw data and create new features. Finally, we got the `train_dp.csv` and `test_dp.csv` files. The feature engineering file is `titanic_preprocessing.py` in the current directory.

The Titanic dataset after preprocessed contains twenty-two features and one label. The features identify the characteristics of individual passengers on titanic. Each feature is stored as a single float number. The label indicates whether an individual passenger survival. The label is stored as an integer and has the possible value of 0, 1(1 survived, 0 deceased)

The meaning of parts of columns in trainSet.csv and testSet.csv is shown as follows:

Column | Explain 
-- | -- 
pclass_* |One-hot value for the “Pclass” field in the raw data.
sex_* | One-hot value for the "Sex" column in the raw data.
embarked_* | One-hot value for the “Embarked” column in the raw data.
title_* |The title identified from the “Name” field.
nosibsp | Constructed from the "SibSp" field to determine if the passenger has siblings/spouses aboard the Titanic.
noparch | Constructed from the "Parch" field to determine if the passenger parents/children aboard the Titanic.
nullcabin | Determine if the "Cabin" field is null（1 Yes, 0 No).
family | Constructed from the "Parch" and "SibSp" and indicated the number of relatives of all families, including himself, siblings, spouses, parents children.
isalone | Indicated whether the passenger is alone or not (1 Yes, 0 No).
ismother | Indicate whether the passenger is a mother or not (1 Yes, 0 No).
realfare | Constructed from the "family" and "Fare" fields, which reveals the actual fare price for each passenger.

We have prepared the titanic dataset `train_dp.csv` and `test_dp.csv` into SQL table `titanic.train` and `titanic.test` separately. We will be using them as training data and test data respectively.

We can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe titanic.train;
```
```sql
%%sqlflow
select *
from titanic.train
limit 1
```

## Train

Let's train a DNNClassifier model. This is a two-category model, we have set three hidden layers and the number of hidden layer units is 10, 40, 10. This can be done by specifying the training clause for SQLFlow's extended syntax.

```text
TRAIN DNNClassifier
WITH
    model.n_classes = 2,
    model.hidden_units = [10, 40, 10]
```

To specify the training data, we use standard SQL statements like ```SELECT * FROM titanic.train```.

We can explicitly specify which column is used for features and which column is used for the label by writing

```text
COLUMN pclass_1, pclass_2, pclass_3, sex_female, sex_male, embarked_c, embarked_q, embarked_s, title_master, title_misc, title_miss, title_mr, title_mrs, nosibsp, noparch, nullcabin, cabinalpha, family, isalone, ismother, age, realfare
LABEL survived
```

At the end of the training process, we save the trained DNN model into table `sqlflow_models.my_dnn_model` as follows:

```text
INTO sqlflow_models.my_dnn_model
```

Putting it all together, we have our first SQLFlow training statement. Users can manually set these parameters for model tuning.

```sql
%%sqlflow
SELECT *
FROM titanic.train
TRAIN DNNClassifier
WITH
  model.n_classes = 2,
  model.hidden_units = [10, 40, 10],
  train.epoch = 200,
  train.batch_size = 64
COLUMN pclass_1, pclass_2, pclass_3, sex_female, sex_male, embarked_c, embarked_q, embarked_s, title_master, title_misc, title_miss, title_mr, title_mrs, nosibsp, noparch, nullcabin, cabinalpha, family, isalone, ismother, age, realfare
LABEL survived
INTO sqlflow_models.my_dnn_model;
```

## Predict

SQLFlow also supports prediction out-of-the-box.

To specify the prediction data, we use standard SQL statements like 

```text
SELECT * FROM titanic.test.
```

Say we want the model, previously stored at sqlflow_models.my_dnn_model, to read the prediction data and write the predicted result into table titanic.predict column Survived. We can write the following SQLFlow prediction statement.

```sql
%%sqlflow
select *
from titanic.test
limit 1
```

```sql
%%sqlflow
SELECT *
FROM titanic.test
predict titanic.predict.survived
USING sqlflow_models.my_dnn_model;
```

```sql
%%sqlflow
SELECT *
FROM titanic.predict
limit 5;
```