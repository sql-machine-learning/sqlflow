# _Design:_ ElasticDL on sqlflow

## Overview

This is a design doc on integration with ElasticDL.

### User Interface

Submitting training job:

```sql
SELECT 
    c1, c2, c3, c4, c5 as class
FROM training_data
TRAIN ElasticDLKerasEstimator
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

1. Based on `SELECT ... from ...`, read ODPS table `<table-name>` and write it to RecordIO files, including both features and labels.
2. Generate model definition file that will be used in `TRAIN` clause, which includes:
    * Model definition `custom_model()` where the model input and output shapes are configured correctly in `inputs = tf.keras.layers.Input(shape=...)` and `outputs = tf.keras.layers.Dense(num_classes)`(based on `SELECT ... LABEL ...`)
    * Pass parameters from `WITH` clause to `custom_model()`'s instantiation
    * Skip `COLUMN` clause for now as this requires additional design details and discussions
    * Pass column names, shapes, and types for features and labels to `dataset_fn`'s feature description that will be used in `tf.io.parse_single_example()` (based on `SELECT ... LABEL ...`)
    * Pass `INTO` clause to ElasticDL's high-level API's `--outputs` arg

#### Prediction

This is similar to evaluation except that prediction results will be written back to an ODPS table through `PREDICT` clause. An additional `PredictionOutputsProcessor` class will be generated in the model definition file for writing the prediction results to ODPS:

```python
class PredictionOutputsProcessor(BasePredictionOutputsProcessor):
    def __init__(self):
        self.odps_writer = ODPSWriter(...)

    def process(self, predictions, worker_id):
        self.odps_writer.from_iterator(...)
```
