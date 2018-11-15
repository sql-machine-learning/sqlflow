# Extended SQL Syntax Design Doc

Our design goal is to make extended SQL syntax cooperate seamlessly with SQL, e.g. supports nested statements.

There are three major machine learning operations: `TRAIN`, `EVAL` and `INFER`. Under the SQL context, we would consider

1. `TRAIN` takes in a table(training data), and produced a trained model.

1. `EVAL` takes in a table(evaluation data) and a trained model, and produced a loss (e.g. accuracy).

1. `INFER` takes in a table and a model, and produce a table (e.g. inferred results).

The first two operations doesn't produce tables, so the rest SQL statement won't reuse its output. 
Therefore we can create a stand alone syntax for them. For example, the following trains a classifer that
predicts the a irisis species based on its sepal_length, sepal_width, petal_length and petal_width.

```SQL
TRAIN DNNClassifier
WITH
  n_classes = 2,
  hidden_units = [10, 20]
INTO my_dnn_model 
SELECT sepal_length, sepal_width, petal_length, petal_width, species
FROM   train_irisis
LIMIT  100
```

Later on, we can evalated the trained model using the following

```SQL
EVAL DNNClassifier
WITH
  n_classes = 2,
  hidden_units = [10, 20]
FROM my_dnn_model 
SELECT sepal_length, sepal_width, petal_length, petal_width, species
FROM   test_irisis
LIMIT  100
```

However, the `INFER` clause needs more care because the rest SQL statement will be reusing its output. One possible
design could be

```SQL
SELECT COUNT(species)
FROM (INFER DNNClassifier
  WITH
    n_classes = 2,
    hidden_units = [10, 20]
  FROM my_dnn_model
  OUTPUT species  /* the column name of the output table */
  SELECT sepal_length, sepal_width, petal_length, petal_width
  FROM   test_irisis
  LIMIT  100)
WHERE species = 1
```

This SQL gives the totally number of irisis that has been classified to be 1. The user could simply think of the inferred output
as an intermediate table.

```SQL
SELECT COUNT(species)
FROM inferred_table
WHERE species = 1
```

