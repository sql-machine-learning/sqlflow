# Customize Data Analysis on SQLFlow Tutorial

Data analysis can help us understand what is in the dataset and the
characteristics of the data.

Data binning is a commonly-used data analysis technique. It groups continous values
into a small number of discretized bins. We will get the distribution of the
data from the binning result.

We can use SQLFlow TO RUN statement to call the SQLFlow runnable which is
released in the form of Docker image. SQLFlow provides some premade runnables
in sqlflow/runnable including the binning runnable. Please use the following
SQL statement to do the data binning. All the table columns specified in the
`--column` parameters will be bucketized to 10 bins.

```sql
%%sqlflow
SELECT * FROM creditcard.creditcard
TO RUN sqlflow/runnable:v0.0.1
CMD "binning.py",
    "--dbname=creditcard",
    "--columns=time,v1,v2,v3,v4,v5,v6,v7,v8,v9,v10,v11,v12,v13,v14,v15,v16,v17,v18,v19,v20,v21,v22,v23,v24,v25,v26,v27,v28,amount",
    "--bin_method=bucket",
    "--bin_num=10"
INTO creditcard_binning_result;
```

The result table contains the binning boundaries, proability distribution for
each bin and also some common used statistical results.

```sql
%%sqlflow
SELECT * FROM creditcard.creditcard_binning_result LIMIT 10;
```

What's more, we can also use `two_dim_binning` runnable to calculate the 2D
distribution for the combination of two variables. In the following SQL
statement, `v1` will bucketized to 10 bins and `v2` will be bucketized to 5
bins.

```sql
%%sqlflow
SELECT * FROM creditcard.creditcard
TO RUN sqlflow/runnable:v0.0.1
CMD "two_dim_binning.py",
    "--dbname=creditcard",
    "--columns=v1,v2",
    "--bin_method=bucket,bucket",
    "--bin_num=10,5"
INTO creditcard_stats_result,creditcard_two_dim_prob,creditcard_two_dim_binning_cumsum_prob;
```
