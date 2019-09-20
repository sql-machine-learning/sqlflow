# Classify Iris Dataset Using DNNClassifer

This tutorial demonstrates how to
1. train a DNNClassifer on iris dataset.
1. use trained DNNClassifer to predict iris class.

## The Dataset

The Iris data set contains four features and one label. The four features identify the botanical characteristics of individual Iris flowers. Each feature is stored as a single float number. The label indicates the class of individual Iris flowers. The label is stored as a integer and has possible value of 0, 1, 2.

We have prepared the iris dataset in table `iris.train` and `iris.test`. We will be using them as training data and test data respectively.

We can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe iris.train;
```

```sql
%%sqlflow
select *
from iris.train
limit 5;
```

## Train

Let's train a DNNClassifier, which has two hidden layers where each layer has ten hidden units. This can be done by specifying the training clause for SQLFlow's extended syntax.

```
TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 20]
```

To specify the training data, we use standard SQL statements like `SELECT * FROM iris.train`.

We explicit specify which column is used for features and which column is used for the label by writing

```
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
```

At the end of the training process, we save the trained DNN model into table `sqlflow_models.my_dnn_model` by writing `INTO sqlflow_models.my_dnn_model`.

Putting it all together, we have our first SQLFlow training statement.

```sql
%%sqlflow
SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 20],
  train.epoch = 100
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

## Predict

SQLFlow also supports prediction out-of-the-box.

To specify the prediction data, we use standard SQL statements like `SELECT * FROM iris.test`.

Say we want the model, previously stored at `sqlflow_models.my_dnn_model`, to read the prediction data and write the predicted result into table `iris.predict` column `class`. We can write the following SQLFlow prediction statement.

```sql
%%sqlflow
SELECT *
FROM iris.test
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
```

After the prediction, we can checkout the prediction result by

```sql
%%sqlflow
SELECT *
FROM iris.predict
LIMIT 5;
```