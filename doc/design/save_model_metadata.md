# Save the Model Metadata After Training

## Background: Why to Save the Model Metadata

Currently, we have saved the model structure and weights after training in the SQLFlow. However, there are some requirements that we have to save the model metadata too.

Let us take an example of the `TO TRAIN` and `TO PREDICT` statements.

```sql
SELECT * FROM my_db.train_table
TO TRAIN my_docker_registry/my_docker_image:latest/MyDNNClassifier
...
LABEL class
INTO my_model;

SELECT * FROM my_db.test_table
TO PREDICT my_db.test_table_prediction.class
USING my_model;
```

- We should save the docker image name which is used in the `TO TRAIN` statement so that we can use the same docker image in the `TO PREDICT` statement.

  The `TO PREDICT` statement should use the same docker image as the `TO TRAIN` statement, i.e., `my_docker_registry/my_docker_image:latest` in the above example. Therefore, we should save the docker image name used in the `TO TRAIN` statement at the end of the training.

- When running `TO PREDICT` statement, we should know whether the trained model is a TensorFlow or XGBoost model, so that we can use different ways to generate the Python code.

  The code generation may be quite different for TensorFlow and XGBoost models. When running the `TO TRAIN` statement, we can use the estimator name (i.e., `MyDNNClassifier` in the above example) to distinguish whether the model to train is a TensorFlow or XGBoost model. But when running the `TO PREDICT` statement, we can only get the trained model name (i.e. `my_model` in the above example) but not the estimator name. Therefore, we do not know whether the model is a TensorFlow or XGBoost model, and we cannot know how to generate the prediction Python code. As a result, we should save the estimator name at the end of the training.

## What Data Should Be Saved As the Model Metadata

We propose to save all fields in the `ir.TrainStmt`, so that we can get all necessary metadata of the `TO TRAIN` statement.

## How to Save the Model Metadata

### Open Source Version
In the open source version, we can save the metadata in the DBMS along with the model structure and weights. Suppose that users write the following SQL statement:

```sql
SELECT * FROM my_db.train_table
TO TRAIN my_docker_registry/my_docker_image:latest/MyDNNClassifier
...
LABEL class
INTO my_db.my_trained_dnn_model;
```

We should save the metadata, model structure and weights in the DBMS table `my_db.my_trained_dnn_model` together. 

In the current implementation, we had saved model structure and weights in the format of:

```
+-----+-----------------------------+
| id  |           block             |
+-----|-----------------------------+
|  0  |                             |
|  1  | model structure and weights |
| ... |                             |
+-----+-----------------------------+
```

The model structure and weights were deserialized into byte stream and saved in multiple lines inside the DBMS table.

In the new design, we propose the saved data in the DBMS table is in the format of:

```
+-----+----------------------------------------------------------+
| id  |                         block                            |
+-----|----------------------------------------------------------+
|  0  |                                                          |
|  1  | (metadata_length, metadata, model structure and weights) |
| ... |                                                          |
+-----+----------------------------------------------------------+
```

The first 64 bit is the metadata's length, then the second field is metadata, and the last field is model structure and weights. This design is almost the same with the current implementation except for the leading metadata fields. In this way, we can get metadata easily without loading the entire model.

### PAI Platform Version
In the PAI platform version, we can save the metadata in the OSS bucket along with the model structure and weights. Suppose that users write the following SQL statement:

```sql
SELECT * FROM my_db.train_table
TO TRAIN my_docker_registry/my_docker_image:latest/MyDNNClassifier
...
LABEL class
INTO my_pai_trained_dnn_model;
```

We propose to save the model metadata in the OSS bucket `oss://sqlflow-models/user_id/my_pai_trained_dnn_model/metadata.json`, and to save the model structure and weights in the OSS bucket `oss://sqlflow-models/user_id/my_pai_trained_dnn_model/model_save`.
