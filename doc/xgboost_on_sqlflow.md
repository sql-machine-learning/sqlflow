# xgboost on sqlflow

## XGBoost 

Gradient boosting machine (GBM) is a widely used (supervised) machine learning method, 
which trains a bunch of weak learners, typically decision trees, 
in a gradual, additive and sequential manner. 
A lot of winning solutions of data mining and machine learning challenges, 
such as : Kaggle, KDD cup, are based on GBM or related techniques.

XGBoost (https://xgboost.ai/) is an optimized distributed gradient boosting library designed to be highly efficient, 
flexible and portable, which is often regarded as one of the best GBM frameworks.


## xgboost on sqlflow via ant-xgboost
   
### Overview

We use [ant-xgboost](https://github.com/alipay/ant-xgboost) as backend,
which is forked from [xgboost](https://github.com/dmlc/xgboost).
_Ant-xgboost_ is nearly same as original xgboost, 
except some improvements to make xgboost easier to use, 
such as better early stopping strategy, parameter checking, and end to end launcher.

### User Experience
    
In terms of sqlflow users, _xgboost_ is an alternative _Estimator_ like _Tensorflow Estimators_. 
Working with xgboost is quite similar to working with Tensorflow estimators; 
just change `TRAIN DNNClassifier` into `TRAIN XGBoostEstimator`. 
In addition, some xgboost specific parameters are required, 
which can be configured in the same way as _Tensorflow_ parameters. 

Below is a demo about training/predicting via xgboost :

```sql
// sample clause of train
select 
    c1, c2, c3, c4, c5 as class
from kaggle_credit_fraud_training_data
TRAIN XGBoostEstimator
WITH
  booster = "gbtree"
  objective = "logistic:binary"
  eval_metric = "auc"
  train_eval_ratio = 0.8
COLUMN
  NUMERIC(c1) as fc1,
  BUCKETIZED(fc1, [0, 10, 100]) as fc2,
  CROSS([fc1, fc2, c3]) as fc3,
  c4 as fc4
LABEL class
INTO sqlflow_models.xgboost_model_table;

// sample clause of predict
select 
    c1, c2, c3, c4
from kaggle_credit_fraud_development_data
PREDICT kaggle_credit_fraud_development_data.class
USING sqlflow_models.xgboost_model_table
COLUMN
  NUMERIC(c1) as fc1,
  BUCKETIZED(fc1, [0, 10, 100]) as fc2,
  CROSS([fc1, fc2, c3]) as fc3,
  c4 as fc4;
```

### Implementation

As `codegen.go` generating _TensorFlow_ code from sqlflow AST,
we will add `codegen_xgboost.go` which translate sqlflow AST into a python launcher program of _xgboost_. 

In _ant-xgboost_, there exists an incubating module named [_xgblauncher_](https://github.com/alipay/ant-xgboost/tree/ant_master/xgboost-launcher), 
an extendable, cloud-native xgboost based machine learning pipeline, 
which provides a python API for building custom `DataSource` and `ModelSource`.

Below is a demonstration of DataSource/ModelSource API.
 
```python
class DataSource:
    """
    DataSource API
    A handler of data reading/writing, which is compatible with both single-machine and distributed runtime.
    """
    def __init__(self, 
                 rank: int, 
                 num_worker: int,
                 column_conf: configs.ColumnFields,
                 source_conf):
        pass
        
    @abstractmethod
    def read(self) -> Iterator[XGBoostRecord]:
        pass

    @abstractmethod
    def write(self, result_iter: Iterator[XGBoostResult]):
        pass

    
class ModelSource:
    """
    ModelSource API
    A handler by which XGBLauncher save/load model(booster) and related information.
    """
    def __init__(self, source_conf):
        pass

    @abstractmethod
    def read_buffer(self, model_path: str) -> bytes:
        pass

    @abstractmethod
    def write_buffer(self, buf: bytes, model_path: str):
        pass

    @abstractmethod
    def read_lines(self, model_path: str) -> List[str]:
        pass

    @abstractmethod
    def write_lines(self, lines: List[str], model_path: str):
        pass
``` 


With _xgblauncher_, we can launch _xgboost_ from sqlflow AST via a lightweight python `code template` and a corrsponding `filler`.
The `code template` roughly includes  components as follows: 

* `TFDataSource` that is responsible for fetching and pre-processing data via tf.feature_columns API.
   Data will be fetched in mini-batch style by executing TF compute graph with mini-batch data feed by `sqlflow_submitter.db.db_generator`.

* `DBDataSource` that is responsible for writing prediction results into specific data base.
   The writing action can be implemented via `sqlflow_submitter.db.insert_values`.

* `LocalModelSource` that is responsible for reading/writing _xgboost_ models on local file system.

* Configuration conversions and entry point of _xgblauncher_.


