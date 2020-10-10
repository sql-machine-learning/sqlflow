# SQLFlow

[![CI](https://github.com/sql-machine-learning/sqlflow/workflows/CI/badge.svg)](https://github.com/sql-machine-learning/sqlflow/actions)
[![codecov](https://codecov.io/gh/sql-machine-learning/sqlflow/branch/develop/graph/badge.svg)](https://codecov.io/gh/sql-machine-learning/sqlflow)
[![GoDoc](https://godoc.org/github.com/sql-machine-learning/sqlflow?status.svg)](https://godoc.org/github.com/sql-machine-learning/sqlflow) 
[![License](https://img.shields.io/badge/license-Apache%202-blue.svg)](LICENSE) 
[![Go Report Card](https://goreportcard.com/badge/github.com/sql-machine-learning/sqlflow)](https://goreportcard.com/report/github.com/sql-machine-learning/sqlflow)

## What is SQLFlow

SQLFlow is a bridge that connects a SQL engine, e.g. MySQL, Hive or [MaxCompute](https://www.aliyun.com/product/odps), with [TensorFlow](https://github.com/tensorflow/tensorflow), [XGBoost](https://github.com/dmlc/xgboost) and other machine learning toolkits. SQLFlow extends the SQL syntax to enable model training, prediction and model explanation.

![](https://github.com/sql-machine-learning/sql-machine-learning.github.io/raw/master/assets/instruction.gif)

## Motivation

The current experience of development ML based applications requires a team of data engineers, data scientists, business analysts as well as a proliferation of advanced languages and programming tools like Python, SQL, SAS, SASS, Julia, R. The fragmentation of tooling and development environment brings additional difficulties in engineering to model training/tuning. What if we marry the most widely used data management/processing language SQL with ML/system capabilities and let engineers with SQL skills develop advanced ML based applications?

There are already some work in progress in the industry. We can write simple machine learning prediction (or scoring) algorithms in SQL using operators like [`DOT_PRODUCT`](https://thenewstack.io/sql-fans-can-now-develop-ml-applications/). However, this requires copy-n-pasting model parameters from the training program to SQL statements. In the commercial world, we see some proprietary SQL engines providing extensions to support machine learning capabilities.

- [Microsoft SQL Server](https://docs.microsoft.com/en-us/sql/advanced-analytics/?view=sql-server-2017): Microsoft SQL Server has the machine learning service that runs machine learning programs in R or Python as an external script.
- [Teradata SQL for DL](https://www.linkedin.com/pulse/sql-deep-learning-sql-dl-omri-shiv): Teradata also provides a RESTful service, which is callable from the extended SQL SELECT syntax.
- [Google BigQuery](https://cloud.google.com/bigquery/docs/bigqueryml-intro): Google BigQuery enables machine learning in SQL by introducing the `CREATE MODEL` statement.

None of the existing solution solves our pain point, instead we want it to be fully extensible.

1. This solution should be compatible to many SQL engines, instead of a specific version or type.
1. It should support sophisticated machine learning models, including TensorFlow for deep learning and [XGBoost](https://github.com/dmlc/xgboost) for trees.
1. We also want the flexibility to configure and run cutting-edge ML algorithms including specifying [feature crosses](https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column), at least, no Python or R code embedded in the SQL statements, and fully integrated with hyperparameter estimation.

## Quick Overview

Here are examples for training a TensorFlow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier) model using sample data Iris.train, and running prediction using the trained model. You can see how cool it is to write some elegant ML code using SQL:

```sql
sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;

...
Training set accuracy: 0.96721
Done training
```

```sql
sqlflow> SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;

...
Done predicting. Predict table : iris.predict
```

## How to use SQLFlow

- [Quick Start](/doc/quick_start.md)
- [Language Guide](/doc/language_guide.md)
- Interactive Examples
    * [DNN Classification example on Iris dataset](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/iris-dnn.ipynb&fileName=iris-dnn.ipynb#/)
    * [DNN Classification example on fraud detection](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/fraud-dnn.ipynb&fileName=fraud-dnn.ipynb#/)
    * [Housing Price Prediction with XGBoost](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/housing-xgboost.ipynb&fileName=housing-xgboost.ipynb#/)
    * [Housing Price Prediction Explanation](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/housing-explain.ipynb&fileName=housing-explain.ipynb#/)
    * [Mathematical Optimization Guide](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/optimization_guide.ipynb&fileName=optimization_guide.ipynb#/)

## Contributing Guidelines

- [Build and Test](/doc/build.md)
- [Walkthrough the Project](/doc/walkthrough.md)

## Roadmap

SQLFlow will love to support as many mainstream ML frameworks and data sources as possible, but we feel like the expansion would be hard to be done merely on our own, so we would love to hear your options on what ML frameworks and data sources you are currently using and build upon. Please refer to our [roadmap](https://github.com/sql-machine-learning/sqlflow/issues/327) for specific timelines, also let us know your current scenarios and interests around SQLFlow project so we can prioritize based on the feedback from the community.

## Feedback

Your feedback is our motivation to move on. Please let us know your questions, concerns, and issues by [filing GitHub Issues](https://github.com/sql-machine-learning/sqlflow/issues).

## License

[Apache License 2.0](https://github.com/sql-machine-learning/sqlflow/blob/develop/LICENSE)

## Published

- An arXiv paper at https://arxiv.org/abs/2001.06846
- Demo Videos
  1. 01/19/2020: https://www.youtube.com/watch?v=qUjQn7ePbto
  1. 10/04/2019: https://www.youtube.com/watch?v=zIkwOQ_davw
  1. 04/01/2019: https://www.youtube.com/watch?v=zIkwOQ_davw
