# ElasticDL on SQLFlow

## Overview

This is a design doc on integration with [ElasticDL](https://github.com/wangkuiyi/elasticdl).

### User Interface

#### Training Job Submission

```sql
SELECT
    c1, c2, c3, c4, c5 as class
FROM training_data
TO TRAIN ElasticDLKerasClassifier
WITH
  optimizer = "optimizer",
  loss = "loss",
  eval_metrics = "eval_metrics_fn",
  num_classes = 10
COLUMN
  c1,
  DENSE(c2, 10),
  BUCKET(c3, [0, 10, 100]),
  c4
LABEL class
INTO trained_elasticdl_keras_classifier;
```

#### Prediction Job Submission

```sql
SELECT
    c1, c2, c3, c4
FROM prediction_data
TO PREDICT prediction_results_table
WITH
  num_classes = 10
USING trained_elasticdl_keras_classifier;
```

#### Run-time Configurations

Users can provide run-time configurations to ElasticDL job via additional parameters with prefix "runtime" within `WITH` clause, for example:

```sql
SELECT
    c1, c2, c3, c4, c5 as class
FROM training_data
TO TRAIN ElasticDLKerasClassifier
WITH
  optimizer = "optimizer",
  loss = "loss",
  eval_metrics = "eval_metrics_fn",
  num_classes = 10,
  runtime.num_epochs = 2,
  runtime.master_resource_request = "cpu=400m,memory=1024Mi",
  runtime.master_resource_limit = "cpu=400m,memory=1024Mi",
  runtime.worker_resource_request = "cpu=400m,memory=2048Mi",
  runtime.worker_resource_limit = "cpu=1,memory=3072Mi",
  runtime.num_minibatches_per_task = 10,
  runtime.num_workers = 2
COLUMN
  c1, c2, c3, c4
LABEL class
INTO trained_elasticdl_keras_classifier;
```

### Implementation Details

#### Training Job

Steps:

1. Based on `SELECT ... FROM ...`, read ODPS table and write it to [RecordIO](https://github.com/wangkuiyi/recordio) files, including both features and labels. These files will be stored in [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/). In the future, we will support reading ODPS table directly without having to convert it to RecordIO files.
2. Generate model definition file (e.g. [cifar10_functional_api.py](https://github.com/wangkuiyi/elasticdl/blob/develop/model_zoo/cifar10_functional_api/cifar10_functional_api.py)) that will be used in `TO TRAIN` clause, which includes:

   - In model definition function e.g. `custom_model()`, we need to configure model input and output shapes correctly in `inputs = tf.keras.layers.Input(shape=<input_shape>)` (only when the model is defined using `tf.keras` functional APIs) and `outputs = tf.keras.layers.Dense(<num_classes>)`(based on `COLUMN ... LABEL ...`). For this MVP, users can provide `<input_shape>` and `<num_classes>` using `WITH` clause which will then get passed to the model constructor `custom_model(input_shape, num_classes)` via `--params` argument in ElasticDL high-level API. In the future, this will be inferred from the ODPS table.
   - Pass additional parameters from `WITH` clause to `custom_model()`'s instantiation, such as `optimizer` and `loss`.
   - Skip support for feature transformation functions such as `DENSE` or `BUCKET` in `COLUMN` clause for now as this requires additional design details and discussions on the use of feature column APIs.
   - Pass column names, shapes, and types for features and labels to `dataset_fn`'s feature description that will be used in `tf.io.parse_single_example()`. For this MVP, column names can be obtained from `SELECT ... LABEL ...`. Each feature columns will be of shape `[1]` and of type `tf.float32` while label column is of shape `[1]` and of type `tf.int64` for classification problems and `tf.float32` for regression problems. In the future, this will be inferred from the ODPS table. An example `dataset_fn()` looks like the following:

   ```python
   def dataset_fn(dataset, mode):
       def _parse_data(record):
           if mode == Mode.PREDICTION:
               feature_description = {
                   "f1": tf.io.FixedLenFeature([1], tf.float32),
                   "f2": tf.io.FixedLenFeature([1], tf.float32),
               }
           else:
               feature_description = {
                   "f1": tf.io.FixedLenFeature([1], tf.float32),
                   "f2": tf.io.FixedLenFeature([1], tf.float32),
                   "label": tf.io.FixedLenFeature([1], tf.int64),
               }
           r = tf.io.parse_single_example(record, feature_description)
           features = {
               "f1": tf.math.divide(tf.cast(r["f1"], tf.float32), 255.0),
               "f2": tf.math.divide(tf.cast(r["f2"], tf.float32), 255.0)
           }
           if mode == Mode.PREDICTION:
               return features
           else:
               return features, tf.cast(r["label"], tf.int32)

       dataset = dataset.map(_parse_data)

       if mode != Mode.PREDICTION:
           dataset = dataset.shuffle(buffer_size=1024)
       return dataset
   ```

   - Pass `INTO` clause to `--outputs` argument in ElasticDL high-level API.

3. Submit ElasticDL training job via a generated ElasticDL high-level API or CLI. Below is an example:

```sh
elasticdl train \
--image_base=elasticdl:ci \
--model_zoo=model_zoo \
--model_def=ElasticDLKerasClassifier \
--training_data=training_table_name \
--evaluation_data=evaluation_table_name \
--num_epochs=2 \
--master_resource_request="cpu=400m,memory=1024Mi" \
--master_resource_limit="cpu=1,memory=2048Mi" \
--worker_resource_request="cpu=400m,memory=2048Mi" \
--worker_resource_limit="cpu=1,memory=3072Mi" \
--minibatch_size=64 \
--num_minibatches_per_task=10 \
--num_workers=2 \
--checkpoint_steps=10 \
--evaluation_steps=15 \
--grads_to_wait=2 \
--job_name=test-mnist \
--log_level=INFO \
--image_pull_policy=Never \
--output=model_output
```

#### Prediction Job

This is similar to training except that prediction results will be written back to an ODPS table through `PREDICT` clause. An additional `PredictionOutputsProcessor` class will be generated in the model definition file for writing the prediction results to ODPS:

```python
class PredictionOutputsProcessor(BasePredictionOutputsProcessor):
    def __init__(self):
        self.odps_writer = ODPSWriter(
            os.environ[ODPSConfig.PROJECT_NAME],
            os.environ[ODPSConfig.ACCESS_ID],
            os.environ[ODPSConfig.ACCESS_KEY],
            os.environ[ODPSConfig.ENDPOINT],
            <prediction_results_table>,
            columns=["f" + str(i) for i in range(<num_classes>)],
            column_types=["double" for _ in range(<num_classes>)],
        )

    def process(self, predictions, worker_id):
        self.odps_writer.from_iterator(...)
```

where an `ODPSWriter` will be instantiated with necessary information on ODPS access and prediction output columns. `<prediction_results_table>` above is inferred from `PREDICT` clause and `<num_classes>` is provided from `WITH` clause.

`USING` clause contains the name to the trained model to be used to make predictions.

#### Differentiate Run-time Configurations

We need to differentiate between the run-time configuration parameters (e.g. `num_workers`, `num_epochs`, etc.) and the model construction parameters (e.g. `optimizer`, `loss`, `num_classes`, etc.). In this MVP, we can add different prefixes to different types of parameters, such as adding "runtime." to run-time configuration parameters.
