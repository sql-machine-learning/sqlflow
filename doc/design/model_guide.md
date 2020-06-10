# SQLFlow Models User Guide

There are various models in SQLFlow Model Zoo, and as the number of SQLFlow users increases,
an important question for users raised -- **How can I choice a model and how can I use it**.
This design provides an outline of the model user guide to help users choose a model and use it in his/her business scene.

## Model Categories

In the first section, we list all the model categories, e.g., Classification, Regression, Unsupervised Clustering,
and time-series. Each type leaves a summary. Each category mapping to one or more Model implementation,
we should list some features—for example, the Classifier model mapping to two model implementations: DNNClassifier and XGBoost.

## Model Implementation

In this section, we show the Model name, model summary, input convention, and accuracy on the public dataset(tutorial).

### Summary

In this section, we can list the advantage and disadvantages of each model implementation.
For example, the XGBoost classifier model can get better accuracy on small datasets and support explanation,
DNNClassifier can get better accuracy on a large dataset but do not support the model explanation.

### Input Data Convention

In this section, we show the input data convention, which includes feature column type and value contract, at least.
Some times, we should pre-process to the original input data, which is very complex, such as word breaker, slide window,
and e.t. We should reference some frequent SQL programs of data pre-processing. 

### Model Parameters

This section is model parameters usage, which tells users when adjustments are needed and the impact of the modifications.

### Model Accuracy

For each Model implementation, we can reference a tutorial, which includes public dataset introduction,
SQLFlow program, and the training and evaluation accuracy. 