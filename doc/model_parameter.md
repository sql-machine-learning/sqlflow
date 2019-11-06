# Model Parameter Document

SQLFlow connects a SQL engine (e.g., MySQL, Hive, or MaxCompute) and TensorFlow and other machine learning toolkits by extending the SQL syntax. The extended SQL syntax contains the WITH clause where a user specifies the parameters of his/her ML jobs. This documentation lists all parameters supported by SQLFlow.

## XGBoost Parameters

### TRAIN

#### Example

```SQL
SELECT * FROM boston.train
TRAIN xgboost.gbtree
WITH
    objective ="reg:squarederror",
    train.num_boost_round = 30,
    validation.select = "SELECT * FROM boston.train LIMIT 8"
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
<tr>
	<td>eta</td>
	<td>Float</td>
	<td>[default=0.3, alias: learning_rate]<br>Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.<br>range: [0,1]</td>
</tr>
<tr>
	<td>num_class</td>
	<td>Int</td>
	<td>Number of classes.<br>range: [2, Infinity]</td>
</tr>
<tr>
	<td>objective</td>
	<td>String</td>
	<td>Learning objective</td>
</tr>
<tr>
	<td>train.num_boost_round</td>
	<td>Int</td>
	<td>[default=10]<br>The number of rounds for boosting.<br>range: [1, Infinity]</td>
</tr>
<tr>
	<td>validation.select</td>
	<td>String</td>
	<td>[default=""]<br>Specify the dataset for validation.<br>example: "SELECT * FROM boston.train LIMIT 8"</td>
</tr>
</table>

### PREDICT

TBD

### EXPLAIN

TBD

