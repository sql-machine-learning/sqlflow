# XGBoost on SQLFlow Tutorial

<a href="https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/housing-xgboost.ipynb&fileName=sqlflow_tutorial_housing_xgboost.ipynb">
  <img alt="Open In PAI-DSW" src="https://pai-public-data.oss-cn-beijing.aliyuncs.com/EN-pai-dsw.svg">
</a>

This is a tutorial on train/predict XGBoost model in SQLFLow, you can find more SQLFlow usage from the [Language Guide](../language_guide.md), in this tutorial you will learn how to:
- Train a XGBoost model to fit the boston housing dataset; and
- Predict the housing price using the trained model;


## The Dataset

This tutorial would use the [Boston Housing](https://www.kaggle.com/c/boston-housing) as the demonstration dataset.
The database contains 506 lines and 14 columns, the meaning of each column is as follows:

Column | Explain 
-- | -- 
crim|per capita crime rate by town.
zn|proportion of residential land zoned for lots over 25,000 sq.ft.
indus|proportion of non-retail business acres per town.
chas|Charles River dummy variable (= 1 if tract bounds river; 0 otherwise).
nox|nitrogen oxides concentration (parts per 10 million).
rm|average number of rooms per dwelling.
age|proportion of owner-occupied units built prior to 1940.
dis|weighted mean of distances to five Boston employment centres.
rad|index of accessibility to radial highways.
tax|full-value property-tax rate per \$10,000.
ptratio|pupil-teacher ratio by town.
black|1000(Bk - 0.63)^2 where Bk is the proportion of blacks by town.
lstat|lower status of the population (percent).
medv|median value of owner-occupied homes in $1000s.

We separated the dataset into train/test dataset, which is used to train/predict our model. SQLFlow would automatically split the training dataset into train/validation dataset while training progress.

```sql
%%sqlflow
describe boston.train;
```

```sql
%%sqlflow
describe boston.test;
```

## Fit Boston Housing Dataset

First, let's train an XGBoost regression model to fit the boston housing dataset, we prefer to train the model for `30 rounds`,
and using `squarederror` loss function that the SQLFLow extended SQL can be like:

```
TO TRAIN xgboost.gbtree
WITH
    train.num_boost_round=30,
    objective="reg:squarederror"
```

`xgboost.gbtree` is the estimator name, `gbtree` is one of the XGBoost booster, you can find more information from [here](https://xgboost.readthedocs.io/en/latest/parameter.html#general-parameters).

We can specify the training data columns in `COLUMN clause`, and the label by `LABEL` keyword:

```
COLUMN crim, zn, indus, chas, nox, rm, age, dis, rad, tax, ptratio, b, lstat
LABEL medv
```

To save the trained model, we can use `INTO clause` to specify a model name:

```
INTO sqlflow_models.my_xgb_regression_model
```

Second, let's use a standard SQL to fetch the training data from table `boston.train`:

```
SELECT * FROM boston.train
```

Finally, the following is the SQLFlow Train statement of this regression task, you can run it in the cell:

```sql
%%sqlflow
SELECT * FROM boston.train
TO TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN crim, zn, indus, chas, nox, rm, age, dis, rad, tax, ptratio, b, lstat
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
```

### Predict the Housing Price
After training the regression model, let's predict the house price using the trained model.

First, we can specify the trained model by `USING clause`: 

```
USING sqlflow_models.my_xgb_regression_model
```

Than, we can specify the prediction result table by `TO PREDICT clause`:

```
TO PREDICT boston.predict.medv
```

And using a standard SQL to fetch the prediction data:

```
SELECT * FROM boston.test
```

Finally, the following is the SQLFLow Prediction statement:

```sql
%%sqlflow
SELECT * FROM boston.test
TO PREDICT boston.predict.medv
USING sqlflow_models.my_xgb_regression_model;
```

Let's have a glance at prediction results.

```sql
%%sqlflow
SELECT * FROM boston.predict;
```
