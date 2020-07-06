# Ant-XGBoost on SQLFlow

**NOTE: ant-xgboost on SQLFlow has moved to [backup_antxgboost_work branch](https://github.com/sql-machine-learning/sqlflow/tree/backup_antxgboost_work)**

## Overview

This is a design doc about why and how to support running ant-xgboost via sqlflow as a machine learning estimator.

We propose to build a lightweight python template for xgboost on basis of `xgblauncher`,
an incubating xgboost wrapper in [ant-xgboost](https://github.com/alipay/ant-xgboost).

## Context

Gradient boosting machine (GBM) is a widely used (supervised) machine learning method, 
which trains a bunch of weak learners, typically decision trees, 
in a gradual, additive and sequential manner. 
A lot of winning solutions of data mining and machine learning challenges, 
such as : Kaggle, KDD cup, are based on GBM or related techniques.

There exists a lot of GBM frameworks (implementations), we propose to use [xgboost](https://xgboost.ai/) as backend of sqlflow, 
which is an optimized distributed gradient boosting library designed to be highly efficient, flexible and portable, 
often regarded as one of the best GBM frameworks.


## _Proposed Solution:_ Ant-XGBoost on SQLFlow
   
We propose to use [ant-xgboost](https://github.com/alipay/ant-xgboost) as backend,
which is consistent with [xgboost](https://github.com/dmlc/xgboost) in kernel level. 
Because in `ant-xgboost`, there exists an incubating module named [xgblauncher](https://github.com/alipay/ant-xgboost/tree/ant_master/xgboost-launcher), 
an extendable, cloud-native xgboost based machine learning pipeline. 
Comparing to python API provided by `xgboost`, it is easier to build a python code template for xgboost task launching on basis of `xgblauncher`.

### User Experience
    
In terms of sqlflow users, xgboost is an alternative `Estimator` like `TensorFlow Estimators`. 
Working with xgboost is quite similar to working with TensorFlow Estimators; just change `TO TRAIN DNNClassifier` into `TO TRAIN XGBoostEstimator`. 

In addition, xgboost specific parameters can be configured in the same way as TensorFlow parameters. 

Below is a demo about training/predicting via xgboost :

```sql
// sample clause of train
select 
    c1, c2, c3, c4, c5 as class
from kaggle_credit_fraud_training_data
TO TRAIN XGBoostEstimator
WITH
  booster = "gbtree"
  objective = "logistic:binary"
  eval_metric = "auc"
  train_eval_ratio = 0.8
COLUMN
  c1,
  DENSE(c2, 10),
  BUCKET(c3, [0, 10, 100]),
  c4
LABEL class
INTO sqlflow_models.xgboost_model_table;

// sample clause of predict
select 
    c1, c2, c3, c4
from kaggle_credit_fraud_development_data
TO PREDICT kaggle_credit_fraud_development_data.class
USING sqlflow_models.xgboost_model_table;
```

### Implementation

As `codegen.go` generating TensorFlow code from sqlflow AST,
we will add `codegen_xgboost.go` which translate sqlflow AST into a python launcher program of xgboost. 

Since xgblauncher provide `DataSource` and `ModelSource`, abstraction of custom I/O pipeline, by which we can reuse data/model pipeline of `runtime`.

The full documentation of xgblauncher will be available soon. Below, we show a demonstration of DataSource/ModelSource API.
 
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


With the help of xgblauncher, we can launch xgboost from sqlflow AST via a lightweight python `code template` and a corresponding `filler`.
The `code template` roughly includes components as follows: 

* `TFDataSource` that is responsible for fetching and pre-processing data via tf.feature_columns API.
   Data will be fetched in mini-batch style by executing TF compute graph with mini-batch data feed by `runtime.db.db_generator`.

* `DBDataSource` that is responsible for writing prediction results into specific data base.
   The writing action can be implemented via `runtime.db.insert_values`.

* `LocalModelSource` that is responsible for reading/writing xgboost models on local file system.

* Configure template building and entry point of xgblauncher.


#### Running Distributed XGBoost Job on K8S Cluster

Distributed training is supported in xgboost via [rabit](https://github.com/dmlc/rabit), a reliable allreduce and broadcast interface for distributed machine learning.
To run a distributed xgboost job with `rabit`, all we need to do is setup a distributed environment.  

For now, xgboost has been bind to some popular distributed computing frameworks, such as Apache Spark, Apache Flink, Dask.
However, specific computing frameworks are not always available in production environments. 
So, we propose a cloud-native approach: running xgboost directly on `k8s cluster`. 
 
As `xgblauncher` is scalable and docker-friendly, xgblauncher-based containers can be easily orchestrated by [xgboost operator](https://github.com/kubeflow/xgboost-operator),
a specific Kubernetes controller for (distributed) xgboost jobs.
With the help of `xgboost operator`, it is easy to handle `XGBoostJob` via `Kubernetes API`, a Kubernetes' custom resource defined by `xgboost operator`. 

`XGBoostJob` building and tracking will be integrated to `xgblauncher` in near future. 
After that, we can generate python codes with an option to decide whether running xgboost job locally or submitting it to remote k8s cluster.
