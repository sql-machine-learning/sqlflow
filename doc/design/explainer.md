# Explain the Machine Learning Model in SQLFlow

## Concept

Although the machine learning model is widely used in many fields, it remains mostly a black box. [SHAP](https://github.com/slundberg/shap) is widely used by data scientists to explain the output of any machine learning model.

This design doc introduces how to support the `Explain SQL` in SQLFlow with SHAP as the backend and display the visualization image to the user.

## User Interface

Users usually use a **TO TRAIN SQL** to train a model and then explain the model using an **TO EXPLAIN SQL**, the simple pipeline like:

Train SQL:

``` sql
SELECT * FROM train_table
TO TRAIN xgboost.Estimator
WITH
    train.objective = "reg:linear"
COLUMN x
LABEL y
INTO my_model;
```

Explain SQL:

``` sql
SELECT * FROM train_table
TO EXPLAIN my_model
WITH
  plots = force 
USING TreeExplainer
```

where:
- `train_table` is the table of training data.
- `my_model` is the trained model.
- `force` and `summary` is the visualized method.
- `TreeExplainer` is the [explain type](https://github.com/slundberg/shap#sample-notebooks).

The **Explain SQL** would display the visualization image on Jupyter like:
<img src="https://raw.githubusercontent.com/slundberg/shap/master/docs/artwork/boston_dataset.png">

## Implement Details

- Enhance the SQLFlow parser to support the `Explain` keyword.
- Implement the `codegen_shap.go` to generate a SHAP Python program. The Python program would be executed by SQLFlow `Executor` module and prints the visualization image in HTML format to stdout. The stdout will be captured by the Go program using [CombinedOutput](https://golang.org/pkg/os/exec/#Cmd.CombinedOutput).
- For each `Explain SQL` request from the SQLFlow magic command, the SQLFlow server would response the HTML text as a single message, and then display the visualization image on Jupyter Notebook

## Note

- For the current milestone, SQLFlow only supports DeepExplainer for the Keras Model, and TreeExplainer for XGBoost, more abundant Explainer and Model type will be supported in the future.
- We don't use the more relevant keyword `Explain` just because `Explain` is [used throughout various SQL databases](https://dzone.com/articles/understanding-mysql-queries-with-explain).
