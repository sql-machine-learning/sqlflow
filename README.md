# SQLFlow

[![Build Status](https://travis-ci.org/sql-machine-learning/sqlflow.svg?branch=develop)](https://travis-ci.org/sql-machine-learning/sqlflow) [![GoDoc](https://godoc.org/github.com/sql-machine-learning/sqlflow?status.svg)](https://godoc.org/github.com/sql-machine-learning/sqlflow) [![Release](https://img.shields.io/github/release/sql-machine-learning/sqlflow.svg)](https://github.com/sql-machine-learning/sqlflow/releases) [![License](https://img.shields.io/badge/license-Apache%202-blue.svg)](LICENSE)

## What is SQLFlow

SQLFlow is a bridge that connects a SQL engine, e.g. MySQL, Hive, SparkSQL or SQL Server, with TensorFlow and other ML toolkits.  SQLFlow extends the SQL language to enable model training, prediction and inference.

## Motivation
The current experience of development ML based application requires a team of data engineer, data scientist, business analyst and a proliferation of advanced langauges and programming tools, for exmaple, Python, SQL, SAS, SASS, Julia, R. The fragmentation of tooling brings additional difficulties in engineering besides algorithmic challenges. What if we marry the most widely used data management/processing language SQL with ML/system capabilities and let engineer with only SQL experience development ML applcations? 

There are some work in progress. We could write simple machine learning prediction (or scoring) algorithms in SQL using operators like [`DOT_PRODUCT`](https://thenewstack.io/sql-fans-can-now-develop-ml-applications/). However, this requires copy-n-pasting model parameters from the training program to SQL statements. In the commercial world, we see some proprietary SQL engines providing extensions to support machine learning capabilities.

 - [Microsoft SQL Server](https://docs.microsoft.com/en-us/sql/advanced-analytics/tutorials/rtsql-create-a-predictive-model-r?view=sql-server-2017): Microsoft SQL Server has the machine learning service that runs machine learning programs in R or Python as an external script.
 - [Teradata SQL for DL](https://www.linkedin.com/pulse/sql-deep-learning-sql-dl-omri-shiv): Teradata also provides a RESTful service, which is callable from the extended SQL SELECT syntax.
 - [Google BigQuery](https://cloud.google.com/bigquery/docs/bigqueryml-intro): Google BigQuery enables machine learning in SQL by introducing the `CREATE MODEL` statement.

None of the existing solution solves our problem, instead we want it fully extensible. 
1. The solution should be compatible to many SQL engines, instead of a specific version or type.
1. It should also support sophisticated machine learning models and toolkits, including TensorFlow for deep learning and [xgboost](https://github.com/dmlc/xgboost) for trees.
1. We also want to build flexibilies to configure and run cutting-edge algorithms, including specifying [feature crosses](https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column), at least, no Python or R code embedded in the SQL statements, and integrate hyperparameter estimation.

## How to Use SQLFlow

- [Installation](doc/installation.md)
- [Running a Demo](doc/demo.md)
- [Extended SQL syntax](doc/syntax.md)

## Contribution

- [Build from source code](doc/build.md).
- [The walkthrough of the source code](doc/walkthrough.md)
- [The choice of parser generator](doc/sql_parser.md)

## Questions and Feedback

Your feedback is our motivation to move on. Please let us know your questions, concerns, and issues by filing Github Issues.

## License

[Apache License 2.0](https://github.com/sql-machine-learning/sqlflow/LICENSE)
