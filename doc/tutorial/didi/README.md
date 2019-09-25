# Introduction

We have prepared test cases that correspond to the three models DNN, XGBoost, and Clustering separately on SQLFlow. Users can test the model on SQLFlow according to the tutorial of each model. Users will learn how to,

- Train a DNN model or XGBoost model using the prepared dataset, then how to predict the label using SQLFlow.
- Use the [SHAP EXPLAINER](https://github.com/slundberg/shap) toolkit to interpret the trained XGBoost model that you can know.
- Predict the patterns of the unlabeled data using the trained Clustering model.

# About DataSet

All data sets are from [Kaggle](https://www.kaggle.com/). We built the DNN model based on the classic [Titanic](https://www.kaggle.com/c/titanic), built the XGBoost model on the [Car Price](https://www.kaggle.com/CooperUnion/cardataset), and built the deep neural network unsupervised clustering model on the [Active Power Consumption](https://www.kaggle.com/uciml/electric-power-consumption-data-set).

# How to use SQLFlow

It's quite simple to try out SQLFlow using [Docker](https://docs.docker.com/).

1. Install [Docker Community Edition](https://docs.docker.com/install/).
2. Run SQLflow by typing the command docker run -it -p 8888:8888 sqlflow/sqlflow:didi.
3. Access localhost:8888 in your Web browser.
4. Open the one of the ipython notebook and run all cells.
