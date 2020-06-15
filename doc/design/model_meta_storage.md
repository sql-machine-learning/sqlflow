# Model Metadata Storage

Model `metadata` in SQLFlow is a piece of data which describes how the model is defined, trained and stored.  It includes the original `train data selection statement`, the `estimator` and its`hyper-parameters` , the `test set performance` and so on. This article describe the data structure of the metadata and how we save and load the metadata to/from kinds of `sqlfs`.

## Background
SQLFlow models can be saved in `MySQL`, `Hive`, `OSS` and other places.  While the model can be trained side by side to the `step` docker image, it can also be trained remote to the step image (as a job on third-party platform).  As a result, we do not have a unified way to store the model currently.  As for the default `submitter` which may use `MySQL` or `Hive` as it data source, we store the model with some metadata into a zipped file, and finally store the file into a database.  In this case, there is only one metadata got saved, that is the `TrainSelect` SQL statement.  As to `pai` submitter, we store the model to `OSS` with more metadata, such as the `Estimator` and the `FeatureColumnNames`.

Some time ago, we have implemented the `SHOW TRAIN` statement, which displays the metadata to user.  Recently, we are developing the Model Zoo.  When releasing a trained model to Model Zoo, we are requested to send the metadata along with the model.  Both features require us to enrich the stored metadata, unify its data structure and make it easy to use.

## The Design
First, we do not save the model metadata from the `step` go code any more.  Because the real training work may be remote to this image.  We move the saving work to the python code which is doing the real training.  A file named `model_meta.json` is dedicated to store the metadata.  Basically, we can serialize all fields in `Train ir` to the file.  Additionally, the evaluation result will be stored if it exists.
```json
{
  "OriginalSQL": "SELECT * from train to TRAIN ...",
  "Estimator": "DNNClassifier",
  "Attributes":{"model.hidden_units":[10,10],"model.n_classes":3,"train.batch_size":1},
  "FeatureColumns":{"feature_columns":[{"FieldDesc":{"name":"sepal_length","dtype":1,"delimiter":"","shape":[1],"is_sparse":false,"vocabulary":null,"MaxID":0}}]},
  "FieldDescs": {"sepal_length": {"name":"sepal_length","dtype":1,"delimiter":"","shape":[1],"is_sparse":false,"vocabulary":null,"MaxID":0}},
  "EvaluationResult": "{auc:0.88}"
  ...
}

```
Then the `model_meta.json` is zipped together with the model data and stored into the database.  As an exception, we do not zip the file on `OSS` storage, we just put the file together this the model data in a directory. In both cases, the directory structure of trained model is like below:
```text
model_dir
  |_ model_meta.json
  |_ model_data
  |_ other files
```

When releasing a trained model to Model Zoo, we can dump the zipped model dir to local file system.  Then extract the metadata using the command:
```bash
tar -xvpf model.tar.gz model_meta.json
```
Other use-cases like the `SHOW TRAIN` statement can follow this way to extract model metadata too.

## Implementation Action

First, we implement this feature for default submitter which store the model in data storage like `MySQL`, `Hive` or `maxcompute`.  Then we implement the feature on `OSS` storage which is not really a database.
