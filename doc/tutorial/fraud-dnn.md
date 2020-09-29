# The Credit Card Fraud Detection Example

<a href="https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/fraud-dnn.ipynb&fileName=sqlflow_tutorial_fraud_dnn.ipynb">
  <img alt="Open In PAI-DSW" src="https://pai-public-data.oss-cn-beijing.aliyuncs.com/EN-pai-dsw.svg">
</a>

The sample data already loaded in MySQL comes from [Kaggle](https://www.kaggle.com/mlg-ulb/creditcardfraud). To train the model using the full dataset, you need to download the dataset and load the dataset into MySQL manually.

You can verify the sample data content in MySQL using:

```sql
%%sqlflow
SELECT * from creditcard.creditcard limit 5;
```

## Train a DNN Model Using SQLFlow

Once your dataset is prepared, you can run the below SQL statement to start training.
Note that SQLFlow will automatically split the dataset into training and validation
sets, the output of evaluation result is calculated using the validation set.

```sql
%%sqlflow
SELECT * from creditcard.creditcard
TO TRAIN DNNClassifier
WITH model.n_classes=2, model.hidden_units=[128,32], train.epoch=100
COLUMN time,v1,v2,v3,v4,v5,v6,v7,v8,v9,v10,v11,v12,v13,v14,v15,v16,v17,v18,v19,v20,v21,v22,v23,v24,v25,v26,v27,v28,amount
LABEL class
INTO creditcard.creditcard_deep_model;
```

## Run Predict

We can use the trained model to predict new data, e.g. we can choose some positive sample in the dataset
to do predict:

```sql
%%sqlflow
SELECT * from creditcard.creditcard
WHERE class=1
TO PREDICT creditcard.predict.class
USING creditcard.creditcard_deep_model;
```

Then we can get the predict result using:

```sql
%%sqlflow
SELECT * from creditcard.predict;
```
