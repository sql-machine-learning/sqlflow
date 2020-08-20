# Get the Model Metadata When Generating Prediction Workflow Codes

## Background

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

The model structure and weights were deserialized into byte stream and saved in multiple rows inside the DBMS table.

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

## How to Get the Model Metadata in Prediction Workflow Codegen

There are 2 situations when we do prediction:

- Case 1: we have trained a model beforehand, and we only run one `TO PREDICT` statement. That is to say, the total SQL statements to run contain only one SQL statement:

  ```sql
  SELECT * FROM my_db.test_table
  TO PREDICT my_db.test_table_prediction.class
  USING my_model;
  ```

  Since we have trained the model `my_model` beforehand, we can get the metadata from the DBMS table or OSS bucket when running these statements above.

- Case 2: the `TO PREDICT` statement uses the trained model from the previous workflow step. That is to say, the total SQL statements to run contain two SQL statements:

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

  Since the the trained model `my_model` will be only generated after the first workflow step runs, we cannot get the model metadata from the DBMS or OSS when we generate the workflow codes for the `TO PREDICT` statement. In this case, we should do some dependency analysis for the SQL statements. That is to say, we should check if there is any `TO TRAIN` statement that will generates the trained model, and get the model metadata from that `TO TRAIN` statement.

In conclusion, the way we try to get the model metadata when generating the workflow codes for the `TO PREDICT` statement is:

- Check if there is any `TO TRAIN` statement that generates the trained model used by the `TO PREDICT` statement.
- If yes, use the metadata from the `TO TRAIN` statement directly.
- If no, try to get the model metadata from the DBMS table or OSS bucket. 