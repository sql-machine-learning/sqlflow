# Proof of Concept: ALPS Submitter

ALPS (Ant Learning and Prediction Suite) provides a common algorithm-driven framework in Ant Financial, focusing on providing users with an efficient and easy-to-use machine learning programming framework and a financial learning machine learning algorithm solution.

This module is used to submit ALPS machine learning training tasks in SQLFlow.

## Precondition
1. For machine learning models, we only consider [TensorFlow premade estimator](https://www.tensorflow.org/guide/premade_estimators).
1. To simplify the design, we only execute `training` without `evaluation` in the estimator.
1. If a table cell is encoded, we assume the user always provides enough decoding information such as dense/sparse, shape via expression such as DENSE, SPARSE

## Data Pipeline        
Standard Select -> Train Input Table -> Decoding -> Input Fn and TrainSpec

The standard select query is executed in SQL Engine like ODPS, SparkSQL, we take the result table as the input of training.

If a table cell is encoded, we assume the user always provides enough decoding information such as dense/sparse, shape, delimiter via expression such as `DENSE`, `SPARSE`.

> The decode expression must exist in `COLUMN` block.

### Dense Format
It is the dense type of encoded data if we have multiple numeric features in a single table cell.

For example, we have numeric features such as `price`, `count` and `frequency`, splice into a string using a comma delimiter.

In this situation, the `DENSE` expression should be used to declare the decoding information such as the shape, delimiter.

```
DENSE(table_column, shape, dtype, delimiter)
Args:
    table_column: column name
    shape(int): shape of dense feature
    delimiter(string): delimiter of encoded feature 
```

### Sparse Format
It is the sparse type of encoded data if we not only have multiple features in a single table cell but also mapping the feature to sparse format.

For example, we have features such as `city`, `gender` and `interest` which has multiple values for each feature. 

The values of `city` are `beijing`, `hangzhou`, `chengdu`.

The values of `gender` are `male` and `female`.

The values of `interest` are `book` and `movie`.

Each of these values has been mapped to an integer and associate with some group.

```text
`beijing` -> group 1, value 1
`hangzhou` -> group 1, value 2
`chengdu` -> group 1, value 3
`male` -> group 1, value 4
`female` -> group 1, value 5
`book` -> group 2, value 1
`movie` -> group 2, value 2
```

If we use colon as the group/value delimiter and use comma as the feature delimiter, "3:1, 4:1, 2:2" means "chengdu, male, movie".

```
SPARSE(table_column, shape, dtype, delimiter, group_delimiter)
Args:
    table_column: column name
    shape(list): list of embedding shape for each group
    delimiter(string): delimiter of feature
    group_delimiter(string): delimiter of value and group
```

### Decoding

The actual decoding action is not happened in this submitter but in ALPS inside.

What we should do here is just generate the configuration file of ALPS.

Let's take an example for training a classifier model for credit card fraud case.

<b>Table Data</b>

| column/row | c1        | c2          | c3 |
|------------|-----------|-------------|----|
| r1         | 10,2,4    | 3:1,4:1,2:2 | 0  |
| r2         | 500,20,10 | 2:1,5:1,2:1 | 1  |

The column `c1` is dense encoded and `c2` is sparse encoded, `c3` is label column.

<b>SQL</b>
```sql
select 
  c1, c2, c3 as class
from kaggle_credit_fraud_training_data
TRAIN DNNClassifier
WITH
  ...
COLUMN
  DENSE(c1, shape=3, dtype=float, delimiter=',')
  SPARSE(c2, shape=[10, 10], dtype=float, delimiter=',', group_delimiter=':')
LABEL class
```
<b>ALPS Configuration</b>

```text
# graph.conf

...

schema = [c1, c2, c3]

x = [
  {feature_name: c1, type: dense, shape:[3], dtype: float, separator:","},
  {feature_name: c2, type: sparse, shape:[10,10], dtype: float, separator:",", group_separator:":"}
]

y = {feature_name: c3, type: dense, shape:[1], dtype: int}

...

```

## Feature Pipeline
Feature Expr -> Semantic Analyze  -> Feature Columns Code Generation -> Estimator

### Feature Expressions 
In SQLFlow, we use Feature Expressions to represent the feature engineering process and convert it into the code snippet using [TensorFlow Feature Column API](https://www.tensorflow.org/guide/feature_columns).

<b>Feature Expressions</b>
```
NUMERIC(key, dtype, shape)

BUCKETIZED(source_column, boundaries)

CATEGORICAL_IDENTITY(key, num_buckets, default_value)

CATEGORICAL_HASH(key, hash_bucket_size, dtype)

CATEGORICAL_VOCABULARY_LIST(key, vocabulary_list, dtype, default_value, num_oov_buckets)

CATEGORICAL_VOCABULARY_FILE(key, vocabulary_file, vocabulary_size, num_oov_buckets, default_value, dtype)

CROSS(keys, hash_bucket_size, hash_key)
```

> The feature expressions must exist in `COLUMN` block.

Here is an example which do `BUCKETIZED` on `c2` then `CROSS` with `c1`.

```sql
select 
    c1, c2, c3 as class
from kaggle_credit_fraud_training_data
TRAIN DNNClassifier
WITH
  ...
COLUMN
  CROSS([NUMERIC(c1), BUCKETIZED(NUMERIC(c2), [0, 10, 100])])
LABEL class

```

### Semantic Analyze
Feature Expressions except for Tensorflow Feature Column API should raise an error.
```sql
/* Not supported */
select * from kaggle_credit_fraud_training_data
TRAIN DNNClassifier
WITH
    ...
COLUMN
    NUMERIC(f1 * 10)
```

### Feature Columns Code Generation
We transform feature columns expression to a code snippet and wrap it as a `CustomFCBuilder` which extends from `alps.feature.FeatureColumnsBuilder`.

Review the above example, the generated code snippet is this:

```python
from alps.feature import FeatureColumnsBuilder

class CustomFCBuilder(FeatureColumnsBuilder):

  def build_feature_columns():
    fc1 = tf.feature_column.numeric_column('c1')
    fc2 = tf.feature_column.numeric_column('c2')
    fc3 = tf.feature_column.bucketized_column(fc2, boundaries = [0, 10, 100])
    fc4 = tf.feature_column.crossed_column([fc2, fc3])
    
    return [fc4]
```

ALPS framework will execute this code snippet and pass the result to the constructor method of the estimator.

### Parameters

We use `WITH` block to set the parameters of training.

If the name is prefixed with `estimator`, it is the parameter of the constructor method of the `Estimator`.

If the name is prefixed with `train_spec`, it is the parameter of the constructor method of the `TrainSpec`.

If the name is prefixed with `input_fn`, it is the parameter of the `input_fn`.

Let's create a DNNClassifier example, the minimum parameters of the constructor method are `hidden_units` and `feature_columns`.

```sql
select 
    c1, c2, c3 as class
from kaggle_credit_fraud_training_data
TRAIN DNNClassifier
WITH
    estimator.hidden_units = [10, 20],
    train_spec.max_steps = 2000,
    input_fn.batch_size = 512
COLUMN
    CROSS([NUMERIC(c1), BUCKETIZED(NUMERIC(c2), [0, 10, 100])])
LABEL class
...
```

For now, we will pass the result of snippet code as `feature_columns` parameters and it will raise an error if the estimator expects it as a different name until `AS` syntax is supported in SQLFlow. 

```sql
select 
    c1, c2, c3, c4, c5 as class
from kaggle_credit_fraud_training_data
TRAIN DNNLinearCombinedClassifier
WITH
  linear_feature_columns = [fc1, fc2]
  dnn_feature_columns = [fc3]
  ...
COLUMN
  NUMERIC(f1) as fc1,
  BUCKETIZED(fc1, [0, 10, 100]) as fc2,
  CROSS([fc1, fc2, f3]) as fc3
LABEL class
...
```