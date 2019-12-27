# Clustering the Active Power Consumption Using Clustering model in SQLFlow

This tutorial describes how to train a Clustering model using the Active Power Consumption dataset.

The [Clustering model](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/clustermodel.md) is designed to support the unsupervised learning model on SQLFlow. In this tutorial, you will learn how to:
- Train a clustering model based on deep embedding neural network on active power consumption dataset;
- Predict the patterns of the unlabeled data using the trained model.
- Clustering the date pairs according to the characteristics of power consumption. We can identify the difference in different categories.

## The Active Power Consumption DataSet
We are using the active [Active Power Consumption Dataset](https://www.kaggle.com/uciml/electric-power-consumption-data-set) data as the demonstration dataset from [kaggle](https://www.kaggle.com/).

The [preprocessed](/doc/tutorial/apsara2019/activepower_clustering/activepower_preprocessing.py) data contains 50 fields. The first field is the date column, and the last field is the pattern to be predicted. The other fields are power consumption data at different times. The time interval is half an hour, and each power consumption data is a numeric feature.

Here are the column descriptions of the dataset:

Column | Explain 
-- | -- 
dates| Date index.
m*| The amount of power consumed in different periods every day. The time interval is half an hour. For example, m1 represents the amount of power consumed from 00:00:00 to 00:30:00.
class| The result group after clustering.

We can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe activepower.train;
```

```sql
%%sqlflow
select * 
from activepower.train 
limit 1;
```

# Train a Clustering model using SQLFlow

Let's train a clustering model.

To specify the data to be clustered, we use standard SQL statements like
```text
SELECT * FROM activepower.train;
```

We can specify the training configurations in the WITH clause. For example, we can set the number of clustering categories to 3, pre-train epochs to 10, etc..
```text
TO TRAIN DeepEmbeddingClusterModel
WITH
    model.n_clusters=3,
    model.pretrain_epochs=10,
    model.train_max_iters=800,
    model.pretrain_lr=1,
    model.train_lr=0.01,
    train.batch_size=256
```

We specify the columns for clustering in the COLUMN clause and the model table name in the INTO clause.
```text
COLUMN m1,m2,m3,m4,m5,m6,m7,m8,m9,m10...
INTO sqlflow_models.my_customized_model
```

Putting it all together, the following is the SQLFlow TO TRAIN statement of this clustering task. You can run it in the cell:
```sql
%%sqlflow
SELECT * FROM activepower.train
TO TRAIN sqlflow_models.DeepEmbeddingClusterModel
WITH
  model.n_clusters=3,
  model.pretrain_epochs=10,
  model.train_max_iters=800,
  model.train_lr=0.01,
  model.pretrain_lr=1,
  train.batch_size=256
COLUMN m1,m2,m3,m4,m5,m6,m7,m8,m9,m10,m11,m12,m13,m14,m15,m16,m17,m18,m19,m20,m21,m22,m23,m24,m25,m26,m27,m28,m29,m30,m31,m32,m33,m34,m35,m36,m37,m38,m39,m40,m41,m42,m43,m44,m45,m46,m47,m48
INTO sqlflow_models.my_customized_model;
```

# Predict the patterns of the data

After training the clustering model, let's predict the patterns of the train data using the trained model.

Firstly, we can use a standard SQL to fetch the prediction data:
```text
SELECT * FROM activepower.train.
```

Next, we can specify the prediction result table by TO PREDICT clause:
```text
TO PREDICT activepower.predict.class
```

Then, we can specify the trained model by USING clause:
```text
USING sqlflow_models.my_customized_model;
```

Putting it all together, the following is the SQLFLow Prediction statement:
```sql
%%sqlflow
SELECT * 
FROM activepower.train
TO PREDICT activepower.predict.class
USING sqlflow_models.my_customized_model;
```

Let's have a glance at prediction results.
```sql
%%sqlflow
SELECT * 
FROM activepower.predict
limit 5;
```

# Analyze the clustering results

We can use the SQL statement to explore the number of each category after clustering.
```sql
%%sqlflow
select class, count(*) as count 
from activepower.predict 
group by class;
```

To explore the differences in power-consuming features among clustered categories, we can use SQL statements to aggregate the average power data at different o'clock.
```sql
%%sqlflow
select 
class
,avg(m1) as clock0
,avg(m8) as clock4
,avg(m16) as clock8
,avg(m24) as clock12
,avg(m32) as clock16
,avg(m40) as clock20
,avg(m46) as clock23
from activepower.predict
group by class;
```
