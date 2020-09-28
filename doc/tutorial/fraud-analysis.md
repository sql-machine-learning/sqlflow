# Customize Data Analysis on SQLFlow Tutorial

Data analysis can help us understand what is in the dataset and the
characteristics of the data.

Data binning is a common used data analysis way. It can group continous values
into a small number of discretized bins. We will get the distribution of the
data from the binning result.

We can use SQLFlow TO RUN statement to execute the runnable which is released in the
form of Docker image. SQLFlow provides some premade runnables in
sqlflow/runnable including the binning runnable. Please use the following SQL
statement to do the data binning.

```SQL
SELECT * FROM creditcard.creditcard
TO RUN sqlflow/runnable:v0.0.1
CMD "binning.py",
    "--dbname=creditcard",
    "--columns=time,v1,v2,v3,v4,v5,v6,v7,v8,v9,v10,v11,v12,v13,v14,v15,v16,v17,v18,v19,v20,v21,v22,v23,v24,v25,v26,v27,v28,amount",
    "--bin_method=bucket",
    "--bin_num=10"
INTO creditcard_binning_result;
```

```SQL
SELECT * FROM creditcard.creditcard_binning_result LIMIT 10;
```

```SQL
SELECT * FROM creditcard.creditcard
TO RUN sqlflow/runnable:v0.0.1
CMD "two_dim_binning.py",
    "--dbname=creditcard",
    "--columns=v1,v2",
    "--bin_method=bucket,bucket",
    "--bin_num=10,5"
INTO creditcard_stats_result,creditcard_two_dim_prob,creditcard_two_dim_binning_cumsum_prob;
```

```SQL
SELECT * FROM creditcard.creditcard_two_dim_prob LIMIT 10;
```

```SQL
SELECT * FROM creditcard.creditcard_two_dim_binning_cumsum_prob LIMIT 10;
```
