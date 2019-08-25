# Design: Analyze the Machine Learning Mode in SQLFlow

## Concept

Although the machine learning model is widely used in many fields, it remains mostly a black box. [SHAP](https://github.com/slundberg/shap) is widely used by data scientists to explain the output of any machine learning model.

This design doc introduces how to support the `Analyze SQL` in SQLFlow with SHAP as the backend and display the visualization image to the user.

## User interface

Users usually use a **TRAIN SQL** to train a model and then analyze the model using an **ANALYZE SQL**, the simple pipeline like:

Train SQL:

``` sql
SELECT * FROM train_table
TRAIN xgboost.Estimator
WITH
    train.objective = "reg:linear"
COLUMN x
LABEL y
INTO my_model;
```

Analyze SQL:

``` sql
SELECT * FROM train_table
ANALYZE my_dnn_model
WITH
  plots = force 
USING TreeExplainer
```

where:
- `train_table` is the table of training data.
- `my_dnn_model` is the trained model.
- `force` and `summary` is the visualized method.
- `TreeExplainer` is the [explain type](https://github.com/slundberg/shap#sample-notebooks).

The **Analyze SQL** would display the visualization image on Jupyter like:
<img src="https://raw.githubusercontent.com/slundberg/shap/master/docs/artwork/boston_dataset.png">

## Implement Details

- Enhance the SQLFlow parser to support the `ANALYZE` keyword.
- implement the `codgen_shap.go` to generate a SHAP Python program to save the visualization image as HTML format.
- For each `ANALYZE SQL` request, the SQLFlow server would response it with the content of the above HTML file, and then the SQLFlow magic command would display it on Jupyter Notebook.

## Note

- For the current milestone, SQLFlow only supports DeepExplainer for the Kerase Model, and TreeExaplainer for the XGboost, more abundant Explainer and Model type will be supported in the future.
- We don't use the more relevant keyword `Explain` just because `Explain` is [used throughout various SQL databases](https://dzone.com/articles/understanding-mysql-queries-with-explain).
