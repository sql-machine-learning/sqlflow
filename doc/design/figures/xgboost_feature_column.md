# Feature Column Support in SQLFlow XGBoost models

## Overall

SQLFlow extends SQL grammar to support data pre-processing using `COLUMN` clauses. 
For example, we can use `CATEGORY_HASH` to parse a string column to an integer column, which is usually a common data pre-processing operation in NLP tasks.

```sql
SELECT string_column1, int_column2, class FROM xgboost.gbtree
TO TRAIN XGBoostModel
COLUMN INDICATOR(CATEGORY_HASH(string_column1, 10)), int_column2
LABEL class 
INTO sqlflow_xgboost_model.my_model;
```

Currently, `COLUMN` clauses are supported in SQLFlow TensorFlow models. The `COLUMN` clauses are transformed
into TensorFlow feature column API calls inside SQLFlow codegen implementation.

However, XGBoost has no similar feature column APIs as TensorFlow. Currently, XGBoost models can only support simple column names like `c1, c2, c3` in `COLUMN` clauses,
and any data pre-processing is not supported. It makes that we cannot use XGBoost to train models which accept string column as their input.

This design explains how SQLFlow supports feature columns in XGBoost model.

## Supported feature column in TensorFlow models

SQLFlow `COLUMN` clauses support the following listed feature columns, and they are implemented by TensorFlow APIs. 

| SQLFlow keywords | TensorFlow API                                              | Description                                                          | 
|------------------|-------------------------------------------------------------|----------------------------------------------------------------------|
| NUMERIC          | tf.feature_column.numeric_column                            | Raw numeric feature column without any pre-processing                | 
| BUCKET           | tf.feature_column.bucketized_column                         | Transform input integer to be the bucket id divided by boundaries |
| CATEGORY_ID      | tf.feature_column.categorical_column_with_identity          | Identity mapping of integer feature column                           |
| CATEGORY_HASH    | tf.feature_column.categorical_column_with_hash_bucket       | Using hash algorithm to map string or integer to category id         |
| SEQ_CATEGORY_ID  | tf.feature_column.sequence_categorical_column_with_identity | Sequence data version of CATEGORY_ID                                 |
| CROSS            | tf.feature_column.crossed_column                            | Combine multiple category features using hash algorithm              |
| INDICATOR        | tf.feature_column.indicator_column                          | Transform category id to multi-hot representation                    |
| EMBEDDING        | tf.feature_column.embedding_column                          | Transform category id to embedding representation                    |

## Feature column design in XGBoost models

The training process of XGBoost model inside SQLFlow are as follows:

- Step 1: Read data from database. Call the `db.db_generator()` method to return a Python generator which yields each row in database. 
- Step 2: Dump SVM file. Call the `dump_dmatrix()` method to write the raw data into an SVM file. This file is ready to be loaded as XGBoost DMatrix.
- Step 3: Training. Load the dumped file to be XGBoost DMatrix and start to train.

Between step 1 and 2, we can do some pre-processing to the raw data from database. In Python side, we can implement the feature column interfaces like:

```Python
class BaseFeatureColumnTransformer(object):
    def __call__(self, inputs):
        raise NotImplementedError()

class NumericColumnTransformer(BaseFeatureColumnTransformer):
    # `column_idx` is the feature index inside `SELECT` statement
    def __init__(self, column_idx):
        self.column_idx = column_idx
    
    # `inputs` are all raw column data
    # NumericColumnTransformer would only take the column indicated by `column_idx`
    def __call__(self, inputs):
        return inputs[self.column_idx]
    
# CategoryColumnTransformer is the base class of all category columns
# This base class is design to do some check. For example, `INDICATOR`
# would only accept category column as its input.
class CategoryColumnTransformer(BaseFeatureColumnTransformer): pass
        
class BucketizedColumnTransformer(CategoryColumnTransformer):
    def __init__(self, column_idx, boundaries, default_value=None):
        self.column_idx = column_idx
        self.boundaries = boundaries
        self.default_value = default_value
        
    def __call__(self, inputs):
        input = inputs[self.column_idx]
        if input < boundaries[0]:
            return 0
            
        for idx, b in enumarate(boudaries):
            if input >= b
                return idx
        return len(boundaries)

class CrossedColumnTransformer(BaseFeatureColumnTransformer):
    def __init__(self, column_indices, hash_bucket_size):
        self.column_indices = column_indices
        self.hash_bucket_size = hash_bucket_size
        
    def _cross(self, transformed_inputs):
        pass

    def __call__(self, inputs):
        selected_inputs = [for idx in self.column_indices]
        self._cross(selected_inputs)
        
class ListedFeatureColumnTransformer(BaseFeatureColumnTransformer):
    def __init__(self, *transformers):
        self.transformers = transformers

    def __call__(self, inputs):
        return [t(inputs) for t in self.transformers]
```

For example, the column clause `COLUMN INDICATOR(CATEGORY_HASH(string_column1, 10)), int_column2` would be finally transformed into Python calls:

```Python
transform_fn = ListedFeatureColumnTransformer(
                    IndicatorColumnTransformer(CategoryColumnWithHashBucketTransformer(column_idx=0, hash_bucket_size=10)),
                    NumericColumnTransformer(column_idx=1) 
                )
```

Then we pass `transform_fn` to `sqlflow_submitter.xgboost.train` method. Inside `sqlflow_submitter.xgboost.train`, we transform the 
raw data from `db.db_generator(...)` by calling `transform_fn.__call__` method. The transformed
data would be writen into SVM file, then it can be loaded in the following train step.

Another concern is that we should perform the same data pre-processing in prediction stage. So we should save the feature columns
of training, so that it can be loaded in prediction stage. Besides, the codegen during prediction stage should also generate the same
transformation codes as training stage.

It should be noticed that `EMBEDDING` is not supported in this design doc. It is because that the `EMBEDDING` feature column may contain
trainable parameters, and these parameters cannot be updated in XGBoost training process.

