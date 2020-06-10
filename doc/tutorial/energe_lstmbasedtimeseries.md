# Time Series Model on SQLFlow Tutorial

This is a tutorial on how to apply a Time Series Model on [energy dataset]( https://www.dropbox.com/s/pqenrr2mcvl0hk9/GEFCom2014.zip?dl=0). 

The dataset is taken from the GEFCom2014 energy forecasting [competition](Tao Hong, Pierre Pinson, Shu Fan, Hamidreza Zareipour, Alberto Troccoli and Rob J. Hyndman, "Probabilistic energy forecasting: Global Energy Forecasting Competition 2014 and beyond", International Journal of Forecasting, vol.32, no.3, pp 896-913, July-September, 2016.). It consists of years of hourly electricity load data from the New England ISO and also includes hourly temperature data. We just choose part of electricity load data whose date range is from 2014-10-01 to 2014-12-31 to complete our tutorial. And, we load the dataset into MySQL manually.

In this notebook, we will demonstrate how to:

- Prepare time series data for training an LSTM forecasting model with SQL
  - Data scaling
  - Reconstruct series data
  - Split the raw data into the train set and test set
- Train an time-series model using the dataset.
- Predict one or more time-step ahead electricity load data, using historical load data only. 

## PART 1  Prepare Data

The energy data set contains two features, one is the date-time column, and the other is electricity load data. 

We can have a quick peek of the raw data by running the following standard SQL statements. 

```sql
%%sqlflow
select * from energy.raw limit 10;
```

```sql
%%sqlflow
select count(*) from energy.raw;
```

### PART 1.1  Scale Data

Because the raw data can't be used directly to build the model, we need to reconstruct the raw data into the series data instead. Before that, we need to scale the data first into the range of (0, 1). We apply the [min-max normalization](https://en.wikipedia.org/wiki/Feature_scaling#Rescaling_(min-max_normalization)) in this project. By the way, it should be noted that data scaling must be done before the data reconstruct shown in the next step.

```sql
%%sqlflow
drop table if exists energy.normalized;
create table energy.normalized as
select
    dt,
    (
        energy - (select min(ta.energy) as a from energy.raw as ta)
    ) / (
        (select max(ta.energy) as a from energy.raw as ta)  
        -(select min(ta.energy) as a from energy.raw as ta)
    ) as energy
from
    energy.raw;
```

Let's have a quick peek of the normalized data.

```sql
%%sqlflow
select * from energy.normalized limit 5;
```

### PART 1.2  Reconstruct Data

Then, you need to choose an appropriate timestep of the series (n_in) and you need to specify how much time-steps target data you want to predict (n_out) by the trained model. By the way, the `n_in` parameter can be tuned manually during the model training stage.

As the following shows, because we want to train a model that `n_in = 10 `and `n_out=4` in the next stage, we reconstruct a time series data that length is 14.  In MySQL, we use the method of `user variables @` to implement the function of Lag function in HIVE. The MySQL Statement is shown as follows.

```sql
%%sqlflow
drop table if exists energy.con_all;
create table energy.con_all as
select * from(
select
    t.dt,
    @lagfield0:= @lagfield1 as col_1,
    @lagfield1:= @lagfield2 as col_2,
    @lagfield2:= @lagfield3 as col_3,
    @lagfield3:= @lagfield4 as col_4,
    @lagfield4:= @lagfield5 as col_5,
    @lagfield5:= @lagfield6 as col_6,
    @lagfield6:= @lagfield7 as col_7,
    @lagfield7:= @lagfield8 as col_8,
    @lagfield8:= @lagfield9 as col_9,
    @lagfield9:= @lagfield10 as col_10,
    @lagfield10:= @lagfield11 as col_11,
    @lagfield11:= @lagfield12 as col_12,
    @lagfield12:= @lagfield13 as col_13,
    @lagfield13:= energy as energy
from
    energy.normalized t,(
        select
            @lagfield0:= null,
            @lagfield1:= null,
            @lagfield2:= null,
            @lagfield3:= null,
            @lagfield4:= null,
            @lagfield5:= null,
            @lagfield6:= null,
            @lagfield7:= null,
            @lagfield8:= null,
            @lagfield9:= null,
            @lagfield10:= null,
            @lagfield11:= null,
            @lagfield12:= null,
            @lagfield13:= null
    ) init
) k where k.col_1 is not null;
```

Let's have a quick peek of the reconstructed data. 

```sql
%%sqlflow
select * from energy.con_all limit 5;
```

```sql
%%sqlflow
select count(*) from energy.con_all;
```

After that, we need to convert the data type of reconstructed data above into float in case some error reported in the later stage.

```sql
%%sqlflow
alter table energy.con_all modify  column col_1 float;
alter table energy.con_all modify  column col_2 float;
alter table energy.con_all modify  column col_3 float;
alter table energy.con_all modify  column col_4 float;
alter table energy.con_all modify  column col_5 float;
alter table energy.con_all modify  column col_6 float;
alter table energy.con_all modify  column col_7 float;
alter table energy.con_all modify  column col_8 float;
alter table energy.con_all modify  column col_9 float;
alter table energy.con_all modify  column col_10 float;
alter table energy.con_all modify  column col_11 float;
alter table energy.con_all modify  column col_12 float;
alter table energy.con_all modify  column col_13 float;
alter table energy.con_all modify  column energy float;
```

### PART 1.3  Split data

In this stage, we separate our dataset into train, validation and test sets, the proportions are 7:2:1. We train the model on the train set. The validation set is used to evaluate the model after each training epoch and ensure that the model is not overfitting the training data. After the model has finished training, we predict the test set by the trained model. 

```sql
%%sqlflow
drop table if exists energy.train;
create table energy.train as
select * from energy.con_all limit 0, 1537;
drop table if exists energy.val;
create table energy.val as
select * from energy.con_all limit 1537, 1976;
drop table if exists energy.test;
create table energy.test as
select * from energy.con_all limit 1976, 2196;
```

## PART 2  Train Model

First, let's train an RNNBasedTimeSeriesModel to fit the energy dataset. the inputs of this model are the dataset have length 10 (`n_in`)series and the output is a 4 (`n_out`) time-steps data.

Due to the output of this task is multi-outputs, we concatenate the target cols into a column. If the output data is one time-step(`n_out=1`), we would do not need the concatenate.The standard SQL statements for specifying the training data like:

```text
SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, col_9,col_10,
concat(col_11,',', col_12,',', col_13,',', energy) as class 
FROM energy.train
```

We can also set the hidden units(`stack_units`) of the LSTM layer and the validation dataset (`validation.select`) and validation function during the train. At the same time, we can set the training parameter like batch_size, verbose, epoch. This can be done by specifying the training clause for SQLFlow's extended syntax.

```text
TO TRAIN sqlflow_models.RNNBasedTimeSeriesModel 
WITH
  model.n_in=10,
  model.stack_units = [500, 500],
  model.n_out=4,
  model.model_type="lstm",
  validation.select = "SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, col_9,col_10,
  concat(col_11,\",\", col_12,\",\",col_13,\",\", energy) as class FROM energy.val",
  train.batch_size=10,
  train.verbose=1,
  train.epoch=60,
  validation.metrics= "MeanAbsoluteError,MeanSquaredError"
```

Then, We explicitly specify which column is used for the label  by writing in the `LABEL clause`and the name  of the saved trained model in the `INTO clause`.

```text
LABEL class
INTO sqlflow_models.my_lstmts_model;
```

Putting it all together, we have the SQLFlow training statement.

```sql
%%sqlflow
SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, col_9,col_10,
concat(col_11,',', col_12,',', col_13,',', energy) as class 
FROM energy.train
TO TRAIN sqlflow_models.RNNBasedTimeSeriesModel 
WITH
  model.n_in=10,
  model.stack_units = [500, 500],
  model.n_out=4,
  model.model_type="lstm",
  validation.select = "SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, 
  col_9,col_10,concat(col_11,\",\", col_12,\",\",col_13,\",\", energy) as class 
  FROM energy.val",
  train.batch_size=10,
  train.verbose=1,
  train.epoch=30,
  validation.metrics= "MeanAbsoluteError,MeanSquaredError"
LABEL class
INTO sqlflow_models.my_lstmts_model;
```

## PART 3  Predict Data

After training the regression model, let's predict the house price using the trained model.

First, we fetch the prediction data using a standard SQL:

```txt
SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, col_9,col_10,
concat(col_11,',', col_12,',', col_13,',', energy) as class 
FROM energy.test
```

Then, we can specify the prediction result table by `TO PREDICT clause`, and  the trained model by `USING clause`:

```
TO PREDICT energy.predict_lstmts_test.class 
USING sqlflow_models.my_lstmts_model;
```

Finally, the following is the SQLFLow Prediction statement:

```sql
%%sqlflow
SELECT col_1, col_2, col_3, col_4, col_5, col_6, col_7, col_8, col_9,col_10,
concat(col_11,',', col_12,',', col_13,',', energy) as class 
FROM energy.test
TO PREDICT energy.predict_lstmts_test.class USING sqlflow_models.my_lstmts_model;
```

Let's have a quick peek of the predicted data.

```sql
%%sqlflow
SELECT * from energy.predict_lstmts_test limit 5;
```

We can see that the target column of the predicted table is a column that consists of multi-column data. In order to get the data of every single column, we need to split the target column by following SQL statement.

```sql
%%sqlflow
drop table if exists energy.predict_lstmts_split;
create table energy.predict_lstmts_split as
SELECT SUBSTRING_INDEX(class, ',', 1) as y_pred1
, SUBSTRING_INDEX(SUBSTRING_INDEX(class, ',', 2), ',', -1) as y_pred2
, SUBSTRING_INDEX(SUBSTRING_INDEX(class, ',', 3), ',', -1) as y_pred3
, SUBSTRING_INDEX(SUBSTRING_INDEX(class, ',', 4), ',', -1) as y_pred4
from energy.predict_lstmts_test;
```

Let's have a quick peek of the split column data.

```sql
%%sqlflow
SELECT * from energy.predict_lstmts_split limit 5;
```

Finally, since our data is a dimensional range between 0 and 1. In order to obtain the original dimension data, we need to denormalize the predicted results, as following SQL statement shows.

```sql
%%sqlflow
select
    y_pred1 * ((select max(energy) from energy.raw) - (select min(energy) from energy.raw)) + (select min(energy) from energy.raw) as y_pred1_raw
    , y_pred2 * ((select max(energy) from energy.raw) - (select min(energy) from energy.raw)) + (select min(energy) from energy.raw) as y_pred2_raw
    , y_pred3 * ((select max(energy) from energy.raw) - (select min(energy) from energy.raw)) + (select min(energy) from energy.raw) as y_pred3_raw
    , y_pred4 * ((select max(energy) from energy.raw) - (select min(energy) from energy.raw)) + (select min(energy) from energy.raw) as y_pred4_raw
from
    energy.predict_lstmts_split 
limit 100;
```

