# Analyzing Model on SQLFlow Tutorial

The [Analyzer](../design/analyzer.md) is designed to explain the machine learning model in SQLFlow. In this tutorial, you will learn how to,

- [Train an XGBoost](housing-xgboost.md) tree model on [the Boston housing dataset](https://www.kaggle.com/c/boston-housing).
- Analyze the trained model using `ANALYZE` SQL statements.

You can find more SQLFlow usages from the [Language Guide](../language_guide.md).

We implement the analyzer based on [SHAP](https://github.com/slundberg/shap). By SQLFlow, an ANALYZE SQL will be translated to the SHAP code. SQLFlow enables the code to read the dataset and load the trained model, then draws a figure to explain the model. At this stage, SQLFlow supports using the [TreeExplianer](https://github.com/slundberg/shap#tree-ensemble-example-with-treeexplainer-xgboostlightgbmcatboostscikit-learn-models) to draw a summary plot.

## Syntax

```sql
SELECT * FROM tbl
ANALYZE a_trained_model
WITH 
  shap_summary.parameter_i=value_i ...
USING TreeExplainer;
```

- A standard SQL statement used to specify the dataset.
- `ANALYZE` introduces the model to explain.
- `USING TreeExplainer` restricts the explainer.
- By `WITH`, we specify the parameters to [summary_plot](https://github.com/slundberg/shap/blob/master/shap/plots/summary.py#L18-L43) with a prefix `shap_summary.`
  like: `shap_summary.plot_type=\"bar\"`.

## The Dataset

We use the [boston housing](https://www.kaggle.com/c/boston-housing) as the demonstration dataset.
First, we train a model to fit the dataset. Next, we write an `ANALYZE` SQL to get an overview of which features are most important for the model.

## Train a Model

```sql
%%sqlflow
SELECT * FROM boston.train
TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN crim, zn, indus, chas, nox, rm, age, dis, rad, tax, ptratio, b, lstat
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
```

## Analyze the Model

We can plot the SHAP values of every feature for every sample.

```sql
%%sqlflow
SELECT *
FROM boston.train
ANALYZE sqlflow_models.my_xgb_regression_model
WITH
    shap_summary.plot_type="dot",
    shap_summary.alpha=1,
    shap_summary.sort=True
USING TreeExplainer;
```

<p align="center">
<img src="https://raw.githubusercontent.com/sql-machine-learning/sqlflow/8e52276a6005f17ccfe6895f7900bf9f79131181/doc/figures/shap_plot_dot.png" width="60%">
</p>

The plot above sorts features by the sum of SHAP value magnitudes over all samples, and use SHAP values to show the distribution of the impacts each feature has on the model output. The color represents the feature values(red high, blue low). This reveals for example that a high LSTAT(% lower status of the population) lowers the predicted home price.

We can also just take the mean absolute value of the SHAP values for each feature to get a standard bar plot:

```sql
%%sqlflow
SELECT *
FROM boston.train
ANALYZE sqlflow_models.my_xgb_regression_model
WITH
    shap_summary.plot_type="bar",
    shap_summary.alpha=1,
    shap_summary.sort=True
USING TreeExplainer;
```

<p align="center">
<img src="https://raw.githubusercontent.com/sql-machine-learning/sqlflow/8e52276a6005f17ccfe6895f7900bf9f79131181/doc/figures/shap_plot_bar.png" width="60%">
</p>
