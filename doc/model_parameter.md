# Model Parameter Document

SQLFlow connects a SQL engine (e.g., MySQL, Hive, or MaxCompute) and TensorFlow and other machine learning toolkits by extending the SQL syntax. The extended SQL syntax contains the WITH clause where a user specifies the parameters of his/her ML jobs. This documentation lists all parameters supported by SQLFlow.

## XGBoost Parameters

### TRAIN

#### Example

```SQL
SELECT * FROM boston.train
TO TRAIN xgboost.gbtree
WITH
    objective ="reg:squarederror",
    train.num_boost_round = 30,
    validation.select = "SELECT * FROM boston.val LIMIT 8"
COLUMN crim, zn, indus, chas, nox, rm, age, dis, rad, tax, ptratio, b, lstat
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
```

#### Parameters

<table>
<tr>
	<td>Name</td>
	<td>Type</td>
	<td>Description</td>
</tr>
</table>

### PREDICT

TBD

### EXPLAIN

TBD

## Tensorflow Parameters

### TRAIN

#### Example

```SQL
SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH
    model.n_classes = 3, model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.my_dnn_model;
```

#### Parameters

<table>
<tr>
	<td>Name</td>
	<td>Type</td>
	<td>Description</td>
</tr>
<tr>
	<td>model.*</td>
	<td>attribute.unknown</td>
	<td>Any model parameters defined in custom models</td>
</tr>
<tr>
	<td>train.batch_size</td>
	<td>int</td>
	<td>[default=1]<br>The training batch size.<br>range: [1,Infinity]</td>
</tr>
<tr>
	<td>train.epoch</td>
	<td>int</td>
	<td>[default=1]<br>Number of epochs the training will run.<br>range: [1, Infinity]</td>
</tr>
<tr>
	<td>train.log_every_n_iter</td>
	<td>int</td>
	<td>[default=10]<br>Print logs every n iterations</td>
</tr>
<tr>
	<td>train.max_steps</td>
	<td>int</td>
	<td>[default=0]<br>Max steps to run training.</td>
</tr>
<tr>
	<td>train.save_checkpoints_steps</td>
	<td>int</td>
	<td>[default=100]<br>Steps to run between saving checkpoints.</td>
</tr>
<tr>
	<td>train.verbose</td>
	<td>int</td>
	<td>[default=0]<br>Show verbose logs when training.<br>possible values: 0, 1</td>
</tr>
<tr>
	<td>validation.metrics</td>
	<td>string</td>
	<td>[default=""]<br>Specify metrics when training and evaluating.<br>example: "Accuracy,AUC"</td>
</tr>
<tr>
	<td>validation.select</td>
	<td>string</td>
	<td>[default=""]<br>Specify the dataset for validation.<br>example: "SELECT * FROM iris.train LIMIT 100"</td>
</tr>
<tr>
	<td>validation.start_delay_secs</td>
	<td>int</td>
	<td>[default=0]<br>Seconds to wait before starting validation.</td>
</tr>
<tr>
	<td>validation.throttle_secs</td>
	<td>int</td>
	<td>[default=0]<br>Seconds to wait when need to run validation again.</td>
</tr>
</table>

### PREDICT

TBD

### EXPLAIN

TBD

