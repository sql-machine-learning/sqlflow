# SQLFlow Models User Guide

There is a various model definition in SQLFlow Model Zoo. The purpose of this document is to design
a user guide to help users select and use a model definition.  This document includes two sections:

1. [Steps to select a model from SQLFlow Model Zoo](/doc/design/model_guide.md#steps-to-select-a-model).
1. [An introduction about how to use a model definition](/doc/design/model_guide.md#how-to-use-a-model).

## Steps to Select a Model

### Select a Model Category Based on Your Purpose

1. Discovery Data Pattern, looking for previously undetected patterns from a data set.  The representative model:

    - [Deep Embedding Clustering](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/deep_embedding_cluster.py) model separates similar points into intuitive groups.

1. Predict values, make forecasts by estimating the relationship between values.  The representative model:

    - [TensorFlow DNNRegression](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnregressor.py) model predict a numerical value.
    - [TensorFlow RNNBasedTimeSeriesModel](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/rnn_based_time_series.py) predicts a sequence time-serious values.

1. Predict categories to classify your data set.  The representative model:

    - [TensorFlow DNNClassifier](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier.py) is a neural network model which can be used on two or multiple categories prediction.

### Select a Model Definition from a Model Category

There are many model definition on each model categories, SQLFlow provides many metrics to help users compare among them
and select one of them as the following:

1. Support Model explanation or not, `YES` means you can use `TO EXPLAIN` SQLFlow syntax to explain a trained model in a figure.
1. Support distributed running or not, distributed job can achieve better performance on a larger dataset.
1. Tutorials, which list many business scenario tutorials written by the SQLFlow program.

## How to Use a Model

For each model definition, there is a description page to introduce how to use it, which includes the following sections
at least.

### Summary

This section describes a model in short, e.g., XGBoost regression model is one of
Gradient Boosting Tree algorithmic implementation, which get better performance and accuracy on small data set.

### Input Data Convention

In this section, we show the input data convention, e.g., should include a label column, which type is numerical,
the input feature column should be a numerical value, and should not be null.

### Model Parameters

In this section, we list all the available parameters, which includes two types:

1. basic parameters, should modify the parameter according to the purpose, e.g. `mdoel.n_classes` specify category number, if the data set contain three categories,
1. advantage parameters, which not users must pay attention, includes
    1. training performance parameters, e.g., scale-up `train.works=3` can achieve better performance but maybe longer waiting time.
    1. optimize model accuracy, e.g., scale-up `hidden_units` may achieve better accuracy but lousy performance.

### Accuracy

The model accuracy on a typical dataset, which could be a tutorial that user can reproduce it.

### Common Preprocessing SQL program

Some models required a complex input data structure. We can list some typical preprocessing SQL program example e.g., sliding window preprocessing is widely used on the time-serious model.
