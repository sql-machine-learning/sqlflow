# Model Meta Storage

This article describe the data structure of the metadata for models trained by SQLFlow and how we save and load it from kinds of `sqlfs`.

## Background
Currently, model can be saved in `MySQL`, `Hive`, `OSS` and other places.  As the model may be trained remote to the step image, we do not have a unified way to store the model.  As for default `submitter` which may use `MySQL` or `Hive` as data source, we store the model with some metadata into a zipped file, and finally store the file into the database.  As to `pai` submitter, we store the model to `OSS` with more metadata.  In the former case, there is only one filed `TrainSelect` in the metadata.

Some time ago, we have implemented the `SHOW TRAIN` statement, which shows the metadata to user. Recently, we are developing the Model Zoo.  When releasing a trained model to Model Zoo, we are requested to send the metadata along the the model data.  Both features require us to enrich the metadata, unify the metadata storage and make it easy to use.

## The Design
First, we do not save the model metadata from step go code any more.  Because the real training work may be remote to the image.  We move the saving work to the python code which is doing the real training.  A file named `model_meta.json` is dedicated to store the metadata.  It is written with the following fields.  Basically, we can serialize all fields in `Train ir` to the file.  Additionally, the evaluation result will be stored if it exists.
```json
{
  "OriginalSQL": "SELECT * from train to TRAIN ...",
  "Estimator": "DNNClassifier",
  "Attributes":{"model.hidden_units":[10,10],"model.n_classes":3,"train.batch_size":1},
  "Features":{"feature_columns":[{"FieldDesc":{"name":"sepal_length","dtype":1,"delimiter":"","shape":[1],"is_sparse":false,"vocabulary":null,"MaxID":0}}]},
  "EvaluationResult": "{auc:0.88}"
  ...
}

```
Then the `model_meta.json` is zipped together with the model data and stored into the database.  The directory structure of trained model is like below:
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
Other use-case like the `SHOW TRAIN` statement can follow this way to extract model metadata too.

## Implementation Action

First, we implement the feature for default submitter which store the model in data storage like `MySQL`, `Hive` and `maxcompute`.  Then we implement the feature on `OSS` storage which is not really a database.
