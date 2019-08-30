# Design: Clustering in SQLflow to analyze patterns in data

## Concept

For analysts and real business people, in the daily analysis work, most of the work is not prediction, but analysis of the patterns in the data. This can help them mine user behavioral characteristics and differences, helping the business discover value and operate.

This design doc introduces how to support the `Cluster Model` in SQLFlow. 

## User interface

Users usually use a **TRAIN SQL** to train a model in Supervised learning. But, In this scenario, we focus on the extraction of data patterns in unsupervised learning. Therefore, we use **EXTRCT SQL** for pattern extraction, the simple pipeline like:

In this scenario, we focus on the extraction of data patterns in unsupervised learning. SO, we plan to use a **TRAIN SQL** to train a unsupervised model. We will support whether to perform Pretrain at the beginning of this unsupervised network in `WITH`, and whether to use the already trained model as a pre-training in `USING`. The simple pipeline like:


TRAIN SQL:

``` sql
SELECT * FROM input_table
TRAIN clusterModel
WITH
    model.encode_units = [100, 7]
    model.n_clusters = 5
    model.run_pretrain = false
COLUMN m1, m2, m3, m4, m5, m6, m7, m8, m9, m10 
USING model.existed_pretrain_model =  existed_pretrain_model
INTO my_cluster_model;
```

PREDICT SQL:

``` sql
SELECT *
FROM input_table
PREDICT output_table
USING my_cluster_model;
```

where:
- `input_table` is the high-dimensional table to be clustered.
- `model.encode_units` is the autoencoder model layer's encoder units, the decode_units can reverse encode_units directly.
- `model.n_clusters` is the number of patterns after clustering.
- `my_cluster_model` is the trained cluster model.
- `run_pretrain`  is used to determine if autoencoder pretrain needs to be run, default true.
- `model.existed_pretrain_model` is used to specify an existing pretrain_model
- `output_table` is the cluster result for input_table, which is adding the `group_id` column predicted by the cluster model to the input_table.

## clusterModel Details
<img src="figures/cluster_model_train_overview.png">

The below figure demonstrates overall workflow for clusterModel train. This figure includes two parts, the pretrian autoencode model and the cluster model are included.
1. First, the former is used to train a pretrain model. The `model.encode_units` describes the layer structure of the encoder of the autoencoder network. We only use the output of the trained encode layer (10000*7) as the input to the clustering model. 
2. Then, the clustering model starts training, randomly initializes weights and multiple iterations, generates clustering models.
3. Finally, the overall train process ultimately outputs an unsupervised clustering model.


## Implement Details
- sqlflow_models/clusterModel.py

```python
class  clusterModel(tf.keras.Model):

	def pre_train(dataset):
			...
			self.autoencoder.fit(dataset)
		pretrainmodel.save(‘/tmp/pretrain.h5’）

	def target_distribution():
		...

	def  cluster_train_loop():
		for ite in range(int(maxiter)):
			if ite % update_interval == 0:
				q = model.predict(x, verbose=0)
				p = target_distribution(q)  # update the auxiliary target distribution p
				y_pred = q.argmax(1)
			idx = index_array[index * batch_size: min((index+1) * batch_size, x.shape[0])]
				loss = model.train_on_batch(x=x[idx], y=p[idx])
			 index = index + 1 if (index + 1) * batch_size <= x.shape[0] else 0
```

- template_tf.go
```python
if 'pre_train' is in classifier:
	classifier.pre_train(InputDataSet)
if 'cluster_train_loop' is in classifier:
	classifier.cluster_train_loop(InputDataSet)

```

## Note

- The user can choose whether to run pre_train before the cluster model, ie run_pretrain=true. The user can also choose to load the already trained model by loading the existed_pretrain_model.
Therefore, there are four cases in total:
1.  run_pretrain = true & Using model.existed_pretrain_model = None：
Autoencoder Pretrain + Random initialization weights for cluster. (Note that model.encode_units `is worked` at this time.)
2.  run_pretrain = true & Using model.existed_pretrain_model = existed_pretrain_model：
existed_pretrain_model Pretrain+ Random initialization weights for cluster. (Note that model.encode_units `is not worked` at this time.)
3.  run_pretrain = false & Using model.existed_pretrain_model = None: 
Random initialization weights for cluster. (Note that model.encode_units `is not worked` at this time.)
4.  run_pretrain = false & Using model.existed_pretrain_model = existed_pretrain_model：
existed_pretrain_model Pretrain+ Random initialization weights for cluster. (Note that model.encode_units `is not worked` at this time.)

- Users can use the trained cluster model in ` PREDICT SQL` to predict the group of input_table to get output_table.
- Finally, the user can perform a combined aggregation operation on the output_table based on the SQL statement to obtain a result_table, which can be saved to the local dataframe and then analyzed according to his own needs.
sometimes, analysts will compare the mean of each feature to analyze the behavioral characteristics and differences of each group of users, maybe by ploting the result_table.

```mysql
%%sqlflow
select 
	group_id
	, avg(m1) as avgm1
	, avg(m2) as avgm2
	, avg(m3) as avgm3
	, avg(m4) as avgm4
	, avg(m5) as avgm5
	, avg(m6) as avgm6
	, avg(m7) as avgm7
	, avg(m8) as avgm8
	, avg(m9) as avgm9
	, avg(m10) as avgm10
from output_table
group by group_id
```

```python
    _.to_dataframes(result_table) 
```

- The example of result_table:

|group_id |  m1  | m2   | m3   | m4   | m5   | m6   | m7   | m8   | m9   | m10  | 
|---------|------|------|------|------|------|------|------|------|------|------|
|    0    | 0.017| 0.015| 0.013| 0.012| 0.01 | 0.01 | 0.009| 0.008| 0.008| 0.008|
|    1    | 0.195| 0.173| 0.154| 0.138| 0.124| 0.111| 0.1  | 0.091| 0.083| 0.076|
|    2    | 0.014| 0.012| 0.011| 0.01 | 0.009| 0.008| 0.007| 0.005| 0.005| 0.004|
|    3    | 0.005| 0.003| 0.003| 0.002| 0.001| 0.001| 0.001| 0.0  | 0.0  | 0.0  |
|    4    | 0.311| 0.291| 0.274| 0.257| 0.24 | 0.224| 0.209| 0.196| 0.185| 0.175|

