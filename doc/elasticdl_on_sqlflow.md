# _Design:_ ElasticDL on sqlflow

## Overview

This is a design doc on integration with [ElasticDL](https://github.com/wangkuiyi/elasticdl).

### User Interface

Submitting training job:

```sql
SELECT 
    c1, c2, c3, c4, c5 as class
FROM training_data
TRAIN ElasticDLKerasClassifier
WITH
  optimizer = "optimizer"
  loss = "loss"
  eval_metrics = "eval_metrics_fn"
COLUMN
  c1,
  NUMERIC(c2, 10),
  BUCKET(c3, [0, 10, 100]),
  c4
LABEL class
INTO sqlflow_models.elasticdl_model_table;
```

Submitting prediction job:
```sql
SELECT 
    c1, c2, c3, c4
FROM prediction_data
PREDICT prediction_results.predicted
USING sqlflow_models.elasticdl_model_table;
```

### Implementation

#### Training

Steps:

1. Based on `SELECT ... FROM ...`, read ODPS table and write it to RecordIO files, including both features and labels.
2. Generate model definition file (e.g. [cifar10_functional_api.py](https://github.com/wangkuiyi/elasticdl/blob/develop/model_zoo/cifar10_functional_api/cifar10_functional_api.py)) that will be used in `TRAIN` clause, which includes:
    * In model definition function e.g. `custom_model()`, we need to configure model input and output shapes correctly in `inputs = tf.keras.layers.Input(shape=<input_shape>)` and `outputs = tf.keras.layers.Dense(<num_classes>)`(based on `COLUMN ... LABEL ...`). For this MVP, users can provide `<input_shape>` and `<num_classes>` using `WITH` clause which will then get passed to the model constructor `custom_model(input_shape, num_classes)` via `--params` argument in ElasticDL high-level API. In the future, this will be inferred from the ODPS table.
    * Pass additional parameters from `WITH` clause to `custom_model()`'s instantiation, such as `optimizer` and `loss`.
    * Skip feature transformation functions such as `NUMERIC` or `BUCKET` in `COLUMN` clause for now as this requires additional design details and discussions on the use of feature column APIs.
    * Pass column names, shapes, and types for features and labels to `dataset_fn`'s feature description that will be used in `tf.io.parse_single_example()`. For this MVP, column names can be obtained from `SELECT ... LABEL ...`. Each feature columns will be of shape `[1]` and of type `tf.float32` while label column is of shape `[1]` and of type `tf.int64` for classification problems and `tf.float32` for regression problems. In the future, this will be inferred from the ODPS table. An example `dataset_fn()` looks like the following:

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
    * Pass `INTO` clause to `--outputs` argument in ElasticDL high-level API.

#### Prediction

This is similar to evaluation except that prediction results will be written back to an ODPS table through `PREDICT` clause. An additional `PredictionOutputsProcessor` class will be generated in the model definition file for writing the prediction results to ODPS:

```python
class PredictionOutputsProcessor(BasePredictionOutputsProcessor):
    def __init__(self):
        self.odps_writer = ODPSWriter(...)

    def process(self, predictions, worker_id):
        self.odps_writer.from_iterator(...)
```
