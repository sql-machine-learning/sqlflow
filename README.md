# SQLFlow

![](https://travis-ci.com/wangkuiyi/sqlflow.svg?token=RA1TUtuBzgTZC3xSQF9x&branch=develop)

## What is SQLFlow

SQLFlow is a small program that connects a SQL engine, e.g., MySQL, Hive, SparkSQL, to a TensorFlow engine.  SQLFlow provides an extended SQL syntax, which can train a model from the result data from a SELECT statement, and makes inference with the data.

## Related Work

We could write simple machine learning prediction (or scoring) algorithms with SQL using operators like [`DOT_PRODUCT`](https://thenewstack.io/sql-fans-can-now-develop-ml-applications/).  However, this requires copy-n-pasting model parameters learned by another program.

Some proprietary SQL engines provide extensions to support machine learning.

Microsoft SQL Server has its [machine learning service](https://docs.microsoft.com/en-us/sql/advanced-analytics/tutorials/rtsql-create-a-predictive-model-r?view=sql-server-2017) that runs machine learning programs in R or Python as an external script:

```sql
CREATE PROCEDURE generate_linear_model
AS
BEGIN
    EXEC sp_execute_external_script
    @language = N'R'
    , @script = N'lrmodel <- rxLinMod(formula = distance ~ speed, data = CarsData);
        trained_model <- data.frame(payload = as.raw(serialize(lrmodel, connection=NULL)));'
    , @input_data_1 = N'SELECT [speed], [distance] FROM CarSpeed'
    , @input_data_1_name = N'CarsData'
    , @output_data_1_name = N'trained_model'
    WITH RESULT SETS ((model varbinary(max)));
END;
```

This extended syntax requires SQL programmers to be capable of programming machine learning algorithms in R or Python.

Teradata extends its SQL engine by providing a RESTful service callable from the extended SQL SELECT syntax.

```sql
SELECT * FROM deep_learning_scorer(
  ON (SELECT * FROM cc_data LIMIT 100)
  URL('http://localhost:8000/api/v1/request')
  ModelName('cc')
  ModelVersion('1')
  RequestType('predict')
  columns('v1', 'v2', ..., 'amount')
)
```

The above syntax couples the deployment of the service (the URL in the above SQL statement) with the algorithm.

Google BigQuery enables machine learning in extended SQL by providing the `CREATE MODEL` statement.

```sql
CREATE MODEL dataset.model_name
  OPTIONS(model_type=’linear_reg’, input_label_cols=[‘input_label’])
AS SELECT * FROM input_table; 
```

Currently, BigQuery only supports two simple models: linear regression and logistic regression.

## Design Goal

None of the above meets our requirement.

We want the system extensible to many SQL engines, e.g., MySQL, SparkSQL, Oracle, SQL Server, Hive.  Therefore, we don't want to build our syntax extension on top of user-defined functions (UDF), which are supposed to write again and again for each SQL engine.

We want the system extensible to sophisticated models, including deep learning and boosting trees.

We want the system able to describe algorithms with top efficiencies, like those winner approaches published on Kaggle.  So, our system should provide enough flexibility like describing [crossed feature columns](https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column).

We want to keep a flat learning curve for our users, which implies that no Python or R coding embedded in the SQL statements.

We understand that a key to address the above challenges is the syntax of the SQL extension. To craft a highly-effective and easy-to-learn syntax, we need user feedback and fast iteration.  Therefore, we'd start from a prototype that supports only MySQL and TensorFlow.  We plan to support more SQL engines and machine learning toolkits later.

## Design Decisions

As the beginning of the iteration, we propose an extension to the SQL SELECT statement.  We are not going a new statement way like that BigQuery provides `CREATE MODEL`, because we want to maintain a loose couple between our system and the underlying SQL engine, and we cannot create the new data type for the SQL engine, like `CREATE MODEL` requires.

We highly appreciate the work of [TensorFlow Estimator](https://www.tensorflow.org/guide/estimators), a high-level API for deep learning. The basic idea behind Estimator is to implement each deep learning model, and related training/testing/evaluating algorithms as a Python class derived from `tf.estimator.Estimator`.  As we want to keep our SQL syntax simple, we would make the system extensible by calling estimators contributed by machine learning experts and written in Python.

The SQL syntax must allow users to set Estimator attributes (parameters of the Python class' constructor, and those of `train`, `evaluate`, or `predict`).  Users can choose to use default values.  We have a plan to integrate our hyperparameter estimation research into the system to optimize the default values.

Though the `tf.estimator.Estimator` utilizes TensorFlow graphs to run the algorithm; our system doesn't restrict the underlying machine learning toolkit to be TensorFlow.  Indeed, as long as an estimator provides methods of `train`, `evaluate`, and `predict`, we don't care if they call TensorFlow or xgboost. The flexibility means that we can use other machine learning toolkits.

We also want to reuse the [feature columns API](https://www.tensorflow.org/guide/feature_columns) from Estimator, which allows users to columns of tables in a SQL engine to features to the model.


## Extended SQL Syntax

Again, just as the beginning of the iteration, we propose the syntax for training as

```sql
SELECT * FROM kaggle_credit_fraud_training_data
LIMIT 1000
TRAIN DNNClassifier       /* a pre-defined TensorFlow estimator, tf.estimator.DNNClassifier */
WITH layers=[100, 200]    /* a parameter of the Estimator class constructor */
     train.batch_size = 8 /* a parameter of the Estimator.train method */
COLUMN *,                 /* all columns as raw features */
       cross(v1, v9, v28) /* plus a derived (crossed) column */
LABEL class
INTO my_model_table;      /* saves trained model parameters and features into a table */
```

We see the redundancy of `*` in two clauses: `SELECT` and `COLUMN`.  The following alternative can avoid the redundancy, but cannot specify the label.

```sql
SELECT *                  /* raw features or the label? */
       corss(v1, v9, v28) /* derived featuers */
FROM kaggle_credit_fraud_training_data
```

Please be aware that we save the trained models into tables, instead of a variable maintained by the underlying SQL engine.  To invent a new variable type to hold trained models, we'd make our system tighly integrated with the SQL engine, and harms the extensibility to other engines.

The result table should include the following information:

1. The estimator name, e.g., `DNNClassifier` in this case.
1. Estimator attributes, e.g., `layer` and `train.batch_size`.
1. The feature mapping, e.g., `*` and `cross(v1, v9, v28)`.

Similarly, to infer the class (fraud or regular), we could

```sql
SELECT * FROM kaggle_credit_fraud_development_data
PREDICT class
USING my_model_table
INTO kaggle_credit_fraud_development_data.class
```

## System Architecture

In the prototype, we use the following architecture:

```
SQL statement -> our SQL parser --standard SQL-> MySQL
                                \-extended SQL-> code generator -> execution engine
```

In the prototype, the code generator generates a Python program that trains or predicts.  In either case, it

1. retrieves the data from MySQL via [MySQL Connector Python API](https://dev.mysql.com/downloads/connector/python/),
1. optionally, retrieves the model from MySQL,
1. trains the model or predicts using the trained model by calling the user specified  TensorFlow estimator,
1. and writes the trained model or prediction results into a table.
