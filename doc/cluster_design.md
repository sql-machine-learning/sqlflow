# Design: Clustering in SQLflow to analyze patterns in data

## Concept

For analysts and real business people, in the daily analysis work, most of the work is not prediction, but analysis of the patterns in the data. This can help them mine user behavioral characteristics and differences, helping the business discover value and operate.

This design doc introduces how to support the `Cluster Model` in SQLFlow. 

## User interface

Users usually use a **TRAIN SQL** to train a model, then use a **Cluster Predict SQL** to predict the clusters and output the results, the simple pipeline like:

Train SQL:

``` sql
SELECT * FROM train_table
TRAIN clusterModel
WITH
	model.encode_units = [100, 7]
    model.n_clusters = 5
COLUMN m1, m2, m3, m4, m5, m6, m7, m8, m9, m10 
INTO my_cluster_model;
```

Cluster Predict SQL:
``` sql
SELECT *
FROM train_table
PREDICT result_table
USING my_cluster_model;
```

where:
- `train_table` is the table of training data.
- `model.encode_units` is the autoencoder layer's encoder units
- `my_cluster_model` is the trained cluster model.
- `model.n_clusters` is the number of patterns after clustering.
- `result_table` is the table of cluster result data.

## Implement Details

- 
- 

## Note
- Train_table is a high-dimensional table to be clustered
- Result_table is a result of averaging the data of each dimension according to the clustering result label.
- Result_table example:
| group_id | m1 | m2 | m3 | m4 | m5 | m6 | m7 | m8 | m9 | m10 |
| ----------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- | --------- |
| 0 | 0.017 | 0.015 | 0.013 | 0.012 | 0.01 | 0.01 | 0.009 | 0.008 | 0.008 | 0.008 |
| 1 | 0.195 | 0.173 | 0.154 | 0.138 | 0.124 | 0.111 | 0.1 | 0.091 | 0.083 | 0.076 |
| 2 | 0.014 | 0.012 | 0.011 | 0.01 | 0.009 | 0.008 | 0.007 | 0.005 | 0.005 | 0.004 |
| 3 | 0.005 | 0.003 | 0.003 | 0.002 | 0.001 | 0.001 | 0.001 | 0.0 | 0.0 | 0.0 |
| 4 | 0.311 | 0.291 | 0.274 | 0.257 | 0.24 | 0.224 | 0.209 | 0.196 | 0.185 | 0.175 |

