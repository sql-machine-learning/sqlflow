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
<tr>
	<td>base_score</td>
	<td>attribute.unknown</td>
	<td>initial prediction score of all instances, global bias.</td>
</tr>
<tr>
	<td>colsample_bylevel</td>
	<td>float32</td>
	<td>Subsample ratio of columns for each level.</td>
</tr>
<tr>
	<td>colsample_bynode</td>
	<td>float32</td>
	<td>Subsample ratio of columns for each split.</td>
</tr>
<tr>
	<td>colsample_bytree</td>
	<td>float32</td>
	<td>Subsample ratio of columns when constructing each tree.</td>
</tr>
<tr>
	<td>eta</td>
	<td>float32</td>
	<td>[default=0.3, alias: learning_rate]<br>Step size shrinkage used in update to prevents overfitting. After each boosting step, we can directly get the weights of new features, and eta shrinks the feature weights to make the boosting process more conservative.<br>range: [0,1]</td>
</tr>
<tr>
	<td>gamma</td>
	<td>float32</td>
	<td>Minimum loss reduction required to make a further partition on a leaf node of the tree.</td>
</tr>
<tr>
	<td>importance_type</td>
	<td>string</td>
	<td>default "gain" The feature importance type for the feature_importances_ property: either "gain", "weight", "cover", "total_gain" or "total_cover".</td>
</tr>
<tr>
	<td>learning_rate</td>
	<td>float32</td>
	<td>Boosting learning rate (xgb's "eta")</td>
</tr>
<tr>
	<td>max_delta_step</td>
	<td>int</td>
	<td>Maximum delta step we allow each tree's weight estimation to be.</td>
</tr>
<tr>
	<td>max_depth</td>
	<td>int</td>
	<td>Maximum tree depth for base learners.</td>
</tr>
<tr>
	<td>min_child_weight</td>
	<td>int</td>
	<td>Minimum sum of instance weight(hessian) needed in a child.</td>
</tr>
<tr>
	<td>missing</td>
	<td>float32</td>
	<td>optional Value in the data which needs to be present as a missing value. If None, defaults to np.nan.</td>
</tr>
<tr>
	<td>n_estimators</td>
	<td>int</td>
	<td>Number of trees to fit.</td>
</tr>
<tr>
	<td>n_jobs</td>
	<td>int</td>
	<td>Number of parallel threads used to run xgboost. (replaces ''nthread'')</td>
</tr>
<tr>
	<td>nthread</td>
	<td>int</td>
	<td>Number of parallel threads used to run xgboost. (Deprecated, please use ''n_jobs'')</td>
</tr>
<tr>
	<td>num_class</td>
	<td>int</td>
	<td>Number of classes.<br>range: [2, Infinity]</td>
</tr>
<tr>
	<td>objective</td>
	<td>string</td>
	<td>Learning objective</td>
</tr>
<tr>
	<td>random_state</td>
	<td>int</td>
	<td>Random number seed. (replaces seed)</td>
</tr>
<tr>
	<td>reg_alpha</td>
	<td>float32</td>
	<td>(xgb's alpha) L1 regularization term on weights</td>
</tr>
<tr>
	<td>reg_lambda</td>
	<td>float32</td>
	<td>(xgb's lambda) L2 regularization term on weights</td>
</tr>
<tr>
	<td>scale_pos_weight</td>
	<td>float32</td>
	<td>Balancing of positive and negative weights.</td>
</tr>
<tr>
	<td>seed</td>
	<td>int</td>
	<td>Random number seed. (Deprecated, please use random_state)</td>
</tr>
<tr>
	<td>silent</td>
	<td>attribute.unknown</td>
	<td>Whether to print messages while running boosting. Deprecated. Use verbosity instead.</td>
</tr>
<tr>
	<td>subsample</td>
	<td>float32</td>
	<td>Subsample ratio of the training instance.</td>
</tr>
<tr>
	<td>train.num_boost_round</td>
	<td>int</td>
	<td>[default=10]<br>The number of rounds for boosting.<br>range: [1, Infinity]</td>
</tr>
<tr>
	<td>validation.select</td>
	<td>string</td>
	<td>[default=""]<br>Specify the dataset for validation.<br>example: "SELECT * FROM boston.train LIMIT 8"</td>
</tr>
<tr>
	<td>verbosity</td>
	<td>int</td>
	<td>The degree of verbosity. Valid values are 0 (silent) - 3 (debug).</td>
</tr>
</table>

### PREDICT

TBD

### EXPLAIN

TBD

