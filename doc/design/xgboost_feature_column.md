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
| DENSE            | tf.feature_column.numeric_column                            | Raw numeric feature column without any pre-processing                | 
| BUCKET           | tf.feature_column.bucketized_column                         | Transform input integer to be the bucket id divided by boundaries    |
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
- Step 3: Training. Load the dumped file to be XGBoost DMatrix and start to train. The training process is performed by calling `xgboost.train` APIs.

As discussed in [COLUMN clause for XGBoost](https://github.com/sql-machine-learning/sqlflow/issues/2190), there are 3 candidate ways to support feature column in XGBoost models:

- Method 1. Perform feature column transformation during step 1 and step 2. Data pre-processing can be done before dumping to 
SVM file. This method is suitable for offline training (both standalone and distributed), prediction and evaluation, since same
transformation Python codes can be generated in both training, prediction and evaluation. But it is not suitable for online
serving, because online serving usually uses other libraries or languages (like C++/Java), which does not support the
transformation codes we generate in SQLFlow Python codes.

- Method 2. Modify the training iteration of XGBoost, and insert transformation codes during each iteration. But it is not
easy to modify the training iteration of XGBoost. Moreover, it is also not suitable for online serving for the same reason 
as method 1.

- Method 3. Combine data pre-processing and model training as a sklearn pipeline. Since sklearn pipeline can be saved as 
a PMML file by [sklearn2pmml](https://github.com/jpmml/sklearn2pmml) or [Nyoka](https://github.com/nyoka-pmml/nyoka), this
method is suitable for standalone training, offline prediction, offline evaluation and online serving. Distributed training 
of sklearn pipeline can be performed using [Dask](https://github.com/dask/dask). However, distributed training pipeline
cannot be saved as a PMML file directly. It is because Dask does not use but mocks native sklearn APIs to build a pipeline,
and these mocked APIs cannot be saved. Another problem is that sklearn pipeline only supports very few data pre-processing 
transformers. For example, hashing a string to a single integer is not supported in sklearn. Of course, we can add more data
pre-processing transformers, but these new added transformers cannot be saved as a PMML file.

In summary, one of the most critical things is how data pre-processing transformers can be saved for online serving. 
After investigating the online serving platform in the company (Arks, etc), data pre-processing steps are usually not saved in 
PMML or Treelite files. The online serving platform provides plugins for users to choose their data pre-processing steps instead
of loading them from PMML or Treelite files. Therefore, we prefer to choose Method 1 to implement feature column in XGBoost models.

The feature column transformers in Python can be implemented as:

```Python
class BaseFeatureColumnTransformer(object):
    def __call__(self, inputs):
        raise NotImplementedError()
        
    def set_column_names(self, column_names):
        self.column_names = column_names

class NumericColumnTransformer(BaseFeatureColumnTransformer):
    # `key` is the column name inside `SELECT` statement
    def __init__(self, key):
        self.key = key
        
    def set_column_names(self, column_names):
        BaseFeatureColumnTransformer.set_column_names(column_names)
        self.index = self.column_names.index(self.key)
    
    # `inputs` are all raw column data
    # NumericColumnTransformer would only take the column indicated by `index`
    def __call__(self, inputs):
        return inputs[self.index]
    
# CategoricalColumnTransformer is the base class of all category columns
# This base class is design to do some check. For example, `INDICATOR`
# would only accept category column as its input.
class CategoricalColumnTransformer(BaseFeatureColumnTransformer): pass
        
class BucketizedColumnTransformer(CategoricalColumnTransformer):
    def __init__(self, key, boundaries, default_value=None):
        self.key = key
        self.boundaries = boundaries
        self.default_value = default_value
        
    def set_column_names(self, column_names):
        BaseFeatureColumnTransformer.set_column_names(column_names)
        self.index = self.column_names.index(self.key)
        
    def __call__(self, inputs):
        input = inputs[self.index]
        if input < boundaries[0]:
            return 0
            
        for idx, b in enumarate(boudaries):
            if input >= b
                return idx
        return len(boundaries)

class CrossedColumnTransformer(BaseFeatureColumnTransformer):
    def __init__(self, keys, hash_bucket_size):
        self.keys = keys
        self.hash_bucket_size = hash_bucket_size
        
    def set_column_names(self, column_names):
        BaseFeatureColumnTransformer.set_column_names(column_names)
        self.column_indices = [self.column_names.index(key) for key in self.keys]
        
    def _cross(self, transformed_inputs): ...

    def __call__(self, inputs):
        selected_inputs = [inputs[idx] for idx in self.column_indices]
        self._cross(selected_inputs)
        
class ComposedFeatureColumnTransformer(BaseFeatureColumnTransformer):
    def __init__(self, *transformers):
        self.transformers = transformers
        
    def set_column_names(self, column_names):
        BaseFeatureColumnTransformer.set_column_names(column_names)
        for t in self.transformers:
            t.set_column_names(column_names)

    def __call__(self, inputs):
        return [t(inputs) for t in self.transformers]
```

For example, the column clause `COLUMN INDICATOR(CATEGORY_HASH(string_column1, 10)), int_column2` would be finally transformed into Python calls:

```Python
transform_fn = ComposedFeatureColumnTransformer(
                    IndicatorColumnTransformer(CategoryColumnWithHashBucketTransformer(key="string_column1", hash_bucket_size=10)),
                    NumericColumnTransformer(key="int_column2") 
                )
```

Then we pass `transform_fn` to `runtime.xgboost.train` method. Inside `runtime.xgboost.train`, we transform the 
raw data from `db.db_generator(...)` by calling `transform_fn.__call__` method. Method `set_column_names` would be called once
when the table schema is obtained in runtime, so that the index of `key` can be inferred in Python runtime. The transformed data 
would be writen into SVM file, then it can be loaded in the following train step.

Another concern is that we should perform the same data pre-processing in prediction/evaluation stage. So we should save the feature columns
of training, so that it can be loaded in prediction/evaluation stage. Besides, the codegen during prediction/evaluation stage should also 
generate the same transformation codes as training stage.

It should be noticed that `EMBEDDING` is not supported in this design doc. It is because that the `EMBEDDING` feature column may contain
trainable parameters, and these parameters cannot be updated in XGBoost training process.

## Export the XGBoost models to PMML/Treelite file

XGBoost supports 2 kinds of APIs to train a model:

- `xgboost.train` . We use this API in our current implementation. The returned Booster can be saved to the format that can be loaded
by Treelite APIs but not by PMML APIs.

- `xgboost.XGBClassifier/XGBRegressor/XGBRanker` . [Sklearn2pmml](https://github.com/jpmml/sklearn2pmml) or 
[Nyoka](https://github.com/nyoka-pmml/nyoka) can only export models built by these APIs to PMML format. But this APIs may be not very easy to use, because:

    - We must distinguish whether the model is a classifier/regressor/ranker beforehand.
    - The constructors of `xgboost.XGBClassifier/XGBRegressor/XGBRanker` mix up the Booster parameters and training parameters together. For example,
      `booster` is one of the Booster parameters, and `n_estimators` is one of the training parameters, but both of them appear in the constructors.
      This makes us hard to distinguish model parameters and training parameters in SQLFlow codes.
    - Names of some of the parameters in `xgboost.XGBClassifier/XGBRegressor/XGBRanker` are different from `xgboost.train` . For example, `n_estimators`
      in `xgboost.XGBClassifier/XGBRegressor/XGBRanker` is the same as `num_boost_round` in `xgboost.train`.
      
Therefore, we prefer to use `xgboost.train` API to perform training in this design, and export PMML/Treelite files in the following ways:

- PMML file can be exported by: 
    - Call `Booster.load_model()` to load trained model.
    - Check the Booster objective to build one of `xgboost.XGBClassifier/XGBRegressor/XGBRanker` .
    - Call `xgboost.XGBClassifier/XGBRegressor/XGBRanker.load_model()` to load the trained model again.
    - Build a sklearn pipeline using the pre-built `xgboost.XGBClassifier/XGBRegressor/XGBRanker` .
    - Save the pipeline using [Sklearn2pmml](https://github.com/jpmml/sklearn2pmml) or [Nyoka](https://github.com/nyoka-pmml/nyoka) as a PMML file.

- Treelite file can be exported by [Model.from_xgboost](https://treelite.readthedocs.io/en/latest/tutorials/import.html#importing-xgboost-models) using the Booster saved by `Booster.save_model()` . 