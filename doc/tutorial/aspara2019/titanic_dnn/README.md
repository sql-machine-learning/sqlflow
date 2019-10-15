# Train and Predict Titanic Dataset Using DNNClassifer in SQLFlow

In this tutorial we will:

1. train a DNNClassifer model using Titanic dataset.
2. use the trained model to predict the class of the passenger's survival status.

## About The Titanic Dataset

The dataset is already loaded in the MySQL service in the docker image, under database `titanic`.
Please refer to [Kaggle](https://www.kaggle.com/c/titanic) for more details about the dataset.
The task is to predict which passenger can survive the tragedy.

We use feature engineering to preprocess the raw data and creating new features.
The feature engineering script is `titanic_preprocessing.py` in the current directory.

The Titanic dataset after preprocessed contains twenty-two features and one label. The features identify the characteristics of individual passengers on titanic. Each feature is stored as a single float number. The label indicates the individual passenger survival. The label is stored as an integer and the possible values are zero and one (one: survived, zero: deceased).

Here are some of the column descriptions of the dataset:

Column | Explain 
-- | -- 
pclass_* | One-hot value for the “Pclass” field in the raw data.
sex_* | One-hot value for the "Sex" column in the raw data.
embarked_* | One-hot value for the “Embarked” column in the raw data.
title_* | The title identified from the “Name” field.
nosibsp | Constructed from the "SibSp" field to determine if the passenger has siblings/spouses aboard the Titanic.
noparch | Constructed from the "Parch" field to determine if the passenger parents/children aboard the Titanic.
nullcabin | Determine if the "Cabin" field is null（1 Yes, 0 No).
family | Constructed from the "Parch" and "SibSp" and indicated the number of relatives of all families, including himself, siblings, spouses, parents children.
isalone | Indicated whether the passenger is alone or not (1 Yes, 0 No).
ismother | Indicate whether the passenger is a mother or not (1 Yes, 0 No).
realfare | Constructed from the "family" and "Fare" fields, which reveals the actual fare price for each passenger.

Table `titanic.train` includes the processed training samples and `titanic.test` includes test samples. We will use them as training data and test data respectively.

We can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe titanic.train;
```
```sql
%%sqlflow
select *
from titanic.train
limit 1;
```

## Train a DNN Model Using SQLFlow

Now let's train a DNNClassifier model. This is a two-category model, we use three hidden layers and the size of hidden layers are ten, forty, ten. This can be done by specifying the training clause using SQLFlow's extended syntax.

```text
TO TRAIN DNNClassifier
WITH
    model.n_classes = 2,
    model.hidden_units = [10, 40, 10]
```

To specify the training data, we use a standard SQL statement like `SELECT * FROM titanic.train`.

We can explicitly specify which column is used as features and which column is used as the label by writing

```text
COLUMN pclass_1, pclass_2, pclass_3, sex_female, sex_male, embarked_c, embarked_q, embarked_s, title_master, title_misc, title_miss, title_mr, title_mrs, nosibsp, noparch, nullcabin, cabinalpha, family, isalone, ismother, age, realfare
LABEL survived
```

At the end of the training process, we want to save the trained DNN model into the table `sqlflow_models.my_dnn_model` as follows:

```text
INTO sqlflow_models.my_dnn_model
```

Putting it all together, we have our SQLFlow training statement. You can modify those parameters to do model tuning.

```sql
%%sqlflow
SELECT *
FROM titanic.train
TO TRAIN DNNClassifier
WITH
  model.n_classes = 2,
  model.hidden_units = [10, 40, 10],
  train.epoch = 200,
  train.batch_size = 64
COLUMN pclass_1, pclass_2, pclass_3, sex_female, sex_male, embarked_c, embarked_q, embarked_s, title_master, title_misc, title_miss, title_mr, title_mrs, nosibsp, noparch, nullcabin, cabinalpha, family, isalone, ismother, age, realfare
LABEL survived
INTO sqlflow_models.my_dnn_model;
```

## Predict Passenger Survival

SQLFlow also supports prediction out-of-the-box.

To specify the prediction data, we use a standard SQL statement like 

```text
SELECT * FROM titanic.test;
```

Say we want to use the model stored at `sqlflow_models.my_dnn_model`, and read the prediction data from table `titanic.test`, and store the prediction result into table `titanic.predict`'s column `survived`. We can write the following SQLFlow prediction statement.

```sql
%%sqlflow
SELECT *
FROM titanic.test
TO PREDICT titanic.predict.survived
USING sqlflow_models.my_dnn_model;

SELECT *
FROM titanic.test
LIMIT 5;

SELECT *
FROM titanic.predict
LIMIT 5;
```

