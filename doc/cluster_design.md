# Design: Clustering in SQLflow to analyze patterns in data

## Concept

For analysts and real business people, in the daily analysis work, most of the work is not prediction, but analysis of the patterns in the data. This can help them mine user behavioral characteristics and differences, helping the business discover value and operate.

This design doc introduces how to support the `Cluster Model` in SQLFlow. 

## User interface

Users usually use a **TRAIN SQL** to train a model in Supervised learning. But, in this scenario, we focus on the extraction of data patterns in unsupervised learning. Therefore, we use **EXTRCT SQL** for pattern extraction, the simple pipeline like:

EXTRCT SQL:

``` sql
SELECT * FROM train_table
EXTRCT clusterModel
WITH
    model.encode_units = [100, 7]
    model.n_clusters = 5
COLUMN m1, m2, m3, m4, m5, m6, m7, m8, m9, m10 
INTO my_cluster_model, result_table;
```

PREDICT SQL:
``` sql
SELECT *
FROM new_table
PREDICT result_test_table
USING my_cluster_model;
```

where:
- `train_table` is the high-dimensional table to be clustered.
- `model.encode_units` is the autoencoder model layer's encoder units, the decode_units can reverse encode_units directly.
- `model.n_clusters` is the number of patterns after clustering.
- `my_cluster_model` is the trained cluster model.
- `result_table` is the cluster result for train_table.
- `new_table`: If you want to apply the model that extracts from train_table to the new data new_table directly, you can use **PREDICT SQL**. Note that the structure of new_table is the same as train_table, namely, same feature column.
- `result_test_table` is the cluster result for new_table.

## Implement Details
-
- 
- 

## Note
- The **EXTRCT SQL** includes two models, the autoencode model and the cluster model. 
First, the former is used to achieve data compression. At training time, the input to this model is train_table (eg. train_table.shape = (10000 * 184)), and the output is also train_table. We only use the output of the trained encode layer (10000*7) as the input to the clustering model.
Then, the clustering model starts training, randomly initializes weights and multiple iterations, generates clustering models and clustering results of train_table.
Next, we average each feature in the train_table according to the clustering result category to get the result table.
Finally, **EXTRCT SQL** outputs two parts, clustering model and the result table are included.
- The example of result_table:
| group_id | m1 | m2 | m3 | m4 | m5 | m6 | m7 | m8 | m9 | m10 |
| ----------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- |
| 0 | 0.017 | 0.015 | 0.013 | 0.012 | 0.01 | 0.01 | 0.009 | 0.008 | 0.008 | 0.008 |
| 1 | 0.195 | 0.173 | 0.154 | 0.138 | 0.124 | 0.111 | 0.1 | 0.091 | 0.083 | 0.076 |
| 2 | 0.014 | 0.012 | 0.011 | 0.01 | 0.009 | 0.008 | 0.007 | 0.005 | 0.005 | 0.004 |
| 3 | 0.005 | 0.003 | 0.003 | 0.002 | 0.001 | 0.001 | 0.001 | 0.0 | 0.0 | 0.0 |
| 4 | 0.311 | 0.291 | 0.274 | 0.257 | 0.24 | 0.224 | 0.209 | 0.196 | 0.185 | 0.175 |

