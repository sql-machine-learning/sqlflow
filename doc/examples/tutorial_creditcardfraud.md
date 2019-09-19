# The Credit Card Fraud Detection Example

The `creditcard_samples.csv` file in this directory comes from [Kaggle](https://www.kaggle.com/mlg-ulb/creditcardfraud). To train the model using the full dataset, you need to download the dataset manually and load the dataset into MySQL under below instructions.

## Import the Dataset into MySQL

Below SQL statements will create a table used to store the dataset:

```sql
%%sqlflow
CREATE DATABASE IF NOT EXISTS creditcard;
CREATE TABLE IF NOT EXISTS creditcard.creditcard (
    time INT,
	v1 float,
	v2 float,
	v3 float,
	v4 float,
	v5 float,
	v6 float,
	v7 float,
	v8 float,
	v9 float,
	v10 float,
	v11 float,
	v12 float,
	v13 float,
	v14 float,
	v15 float,
	v16 float,
	v17 float,
	v18 float,
	v19 float,
	v20 float,
	v21 float,
	v22 float,
	v23 float,
	v24 float,
	v25 float,
	v26 float,
	v27 float,
	v28 float,
	amount float,
	class INT);
```

Then we could run a python script to insert values to that table:

```python
from MySQLdb import connect
fn = open("creditcard_samples.csv", "r")
line_no = 0
samples = []
for line in fn:
    if line_no == 0:
        line_no += 1
        continue
    l = line.replace("\n", "")
    samples.append("(%s)" % l)
    line_no += 1
sql = "INSERT INTO creditcard.creditcard VALUES %s" % ",".join(samples)
conn = connect(user="root", passwd="root", db="creditcard", host="localhost", port=3306)
c = conn.cursor()
c.execute(sql)
conn.commit()
c.close()
conn.close()
```

You can verify the data in MySQL using:

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
TRAIN DNNClassifier
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
PREDICT creditcard.predict.class
USING creditcard.creditcard_deep_model;
```

Then we can get the predict result using:

```sql
%%sqlflow
SELECT * from creditcard.predict;
```
