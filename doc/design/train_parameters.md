# Information Necessary for Code Generators in SQLFlow

SQLFlow extends the syntax of the SELECT statement of SQL to support training a model:
```sql
SELECT * FROM kaggle_credit_fraud_training_data
LIMIT 1000
TO TRAIN DNNClassifier       /* a pre-defined TensorFlow estimator, tf.estimator.DNNClassifier */
WITH layers=[100, 200],   /* a parameter of the Estimator class constructor */
     train.batch_size = 8 /* a parameter of the Estimator.train method */
COLUMN *,                 /* all columns as raw features */
       cross(v1, v9, v28) /* plus a derived (crossed) column */
LABEL class
INTO sqlflow_models.my_model_table;      /* saves trained model parameters and features into a table */
```

Currently, we have the following syntax allowing users to provide necessary information for the training.
```sql
SELECT STATEMENT
TO TRAIN ...
WITH ...
COLUMN ... [FOR ...]
LABEL ...
INTO ...
```

SQLFlow server passes the above information to code generators like `sql/codegen.go` and `sql/codegen_alps.go`, which generates the training program may be using different and even conflicting rules.

Things will be even more difficult if we got other kinds of `sql/codegen_**.go` in the future. 

In this document, we summarize information necessary for the code generators.

## Necessary Information for Training

### Model Name
Model Name is a string written after the keyword of `TO TRAIN`, which can be the name of a [TensorFlow pre-made estimator](https://www.tensorflow.org/guide/premade_estimators) or the full package path of a customized Estimator/KerasModel.

If the model name is the full package path of a customized model, the model should be installed according to [How to install the customized model in SQLFlow]().

### Model Constructor Parameters
The model will be constructed by `codegen` according to the module name, and the constructor parameters can be set in the `WITH` block.

Here are some rules:
1. Name of a parameter must begin with `model.` and we take the rest of it as the real name.
2. To simplify implementation, the value of a parameter must be a type of numeric, string or list.

Take an example for `DNNClassifier`
```sql
TO TRAIN DNNClassifier
WITH
    model.hidden_units = [10, 20, 10]
```

### Training Hyper-Parameters
The training hyper-parameters can be also set in the `WITH` block.

<b>Here is the list of hyper-parameters planning to support.</b>

|                  |      parameter name      | type of value | default value | comment                                                                                                                     |
|------------------|:---------------------:|:-------------:|:-------------:|-----------------------------------------------------------------------------------------------------------------------------|
| batch size       | train.batch_size      | integer       | 512           |                                                                                                                             |
| drop remainder   | train.drop_remainder  | bool          | true          |                                                                                                                             |
| cache            | train.cache           | bool/string   | false         | bool: enable cache in memory or not. string: enable cache and the name of a directory on the filesystem to caching tensors. |
| epoch            | train.epoch           | integer       | 1             |                                                                                                                             |
| shard            | train.shard           | integer       | 1             | distributed training if greater than 1. Fixed size of PS btw.                                                               |
| shuffle          | train.shuffle         | bool/integer  | false         | bool: enable shuffle or not. integer: shuffle buffer size.                                                                  |
| max_steps        | train.max_steps       | integer       | None          |                                                                                                                             |
| eval steps       | eval.steps            | integer       | None          |                                                                                                                             |
| start delay secs | eval.start_delay_secs | integer       | 120           |                                                                                                                             |
| throttle_secs    | eval.throttle_secs    | integer       | 600           |                                                                                                                             |

### COLUMN FOR
The expressions in `COLUMN` contains the `Feature Columns` information. There could be multiple `COLUMN` blocks in the SQL. 

The value of `FOR` keyword represents which parameter of the constructor method the `feature columns` assigning to.

For example, the following SQL
```sql
...
TO TRAIN
	DNNLinearCombinedClassifier
WITH
	...
COLUMN
	DENSE(...) FOR linear_feature_columns
COLUMN
	BUCKET(...) FOR dnn_feature_columns
...
```
will be translated to 
```python
	estimator = tf.estimator. DNNLinearCombinedClassifier(
		linear_feature_columns = [tf.feature_column.numeric(...)],
		dnn_feature_columns = [tf.feature_column.bucket(...)],
		...
	)
```

They were not only translated into [TensorFlow Feature Columns](https://www.tensorflow.org/guide/feature_columns), but also contains the encoding information of data such as `DENSE` or `SPARSE`. 

For example, the following SQL
```sql
SELECT 
...
COLUMN
	DENSE(c1, ...)
	SPARSE(c2, ...)
```
represents that the `c1` field is the dense format and the `c2` field is the sparse format.

<b>Here is a list of supported expression:</b>

| expression | arguments of expression                                          | example                            |
|------------|------------------------------------------------------------------|------------------------------------|
| dense      | 1. field name (str) <br> 2. dense shape (list of integer) <br> 3. separator (str)  | dense(c1, [100, 200], comma)              |
| sparse     | 1. field name (str) <br> 2. sparse shape (integer) <br> 3. separator (str) | sparse(c2, 10000, comma)           |
| bucket     | 1. key (numeric) <br> 2. bucket size (integer)                        | bucket(numeric(c1, 100), 20)       |
| cat_id     | 1. field name (str) <br> 2. bucket size (integer)                     | cat_id(c2, 10000)                  |
| embedding  | 1. key (cat_id) <br> 2. dimension (integer) <br> 3. combiner (str)         | embedding(cat_id(c2, 10000), mean) |


## Independent Module for Resolving of Training Parameters
Since the format of training parameters has been unified, it's better to have an independent module in SQLFlow to do the resolving according to the rules instead of doing it in each `codegen_**.go`.

The advantage of an independent module contains
1. Unifying resolving process to avoid differences in processing between `codegen` modules. 
2. Fast fail during resolving before the code generation.

Here we propose a module such as `resolving.go` which take the `trainClause` as input and outputs a struct named `resolvedTrainClause` for all kinds of `codegen_**.go` to do code generation.

The struct named `resolvedTrainClause` looks like this
```go
package sql

type resolvedTrainClause struct {
	IsPreMadeModel  bool
	ModelName   string
	ModelConstructorParameters map[string]interface{}
	BatchSize int
	DropRemainder bool
	EnableCache bool
	CachePath string
	Epoch int
	Shard int
	EnableShuffle bool
	ShuffleBufferSize int
	MaxSteps int
	EvalSteps int
	EvalStartDelay int
	EvalThrottle int
	FeatureColumns map[string][]featureColumn
	FeatureSpecs map[string][]featureSpec
}
```

