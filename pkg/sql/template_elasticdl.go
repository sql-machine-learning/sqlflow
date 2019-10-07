// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

// TODO: Right now this is all generated by codegen_elasticdl.go
// Need to revisit so the model definition can actually be obtained
// from model zoo.
const elasticdlModelDefTemplateText = `
import os

import tensorflow as tf

from elasticdl.python.common.constants import Mode
from elasticdl.python.common.log_util import default_logger as logger
from elasticdl.python.common.odps_io import ODPSWriter
from elasticdl.python.worker.prediction_outputs_processor import (
    BasePredictionOutputsProcessor,
)


def custom_model():
    inputs = tf.keras.layers.Input(shape=({{.InputShape}}, 1), name="input")
    outputs = tf.keras.layers.Dense({{.OutputShape}}, name="output")(inputs)
    return tf.keras.Model(inputs=inputs, outputs=outputs, name="simple-model")


def loss(output, labels):
    return tf.reduce_sum(tf.reduce_mean(tf.reshape(output, [-1])) - labels)


def optimizer(lr=0.1):
    return tf.optimizers.SGD(lr)


def dataset_fn(dataset, mode, metadata):
    def _parse_data(record):

        def _get_features_without_labels(
            record, label_col_ind, features_shape
        ):
            features = [
                record[:label_col_ind],
                record[label_col_ind + 1 :],  # noqa: E203
            ]
            features = tf.concat(features, -1)
            return tf.reshape(features, features_shape)

        features_shape = ({{.InputShape}}, 1)
        labels_shape = (1,)
        {{if .IsTraining}}
        label_col_name = "{{.LabelColName}}"
        if mode != Mode.PREDICTION:
            if label_col_name not in metadata.column_names:
                raise ValueError(
                    "Missing the label column '%s' in the retrieved "
                    "table." % label_col_name
                )
            label_col_ind = metadata.column_names.index(label_col_name)
            labels = tf.reshape(record[label_col_ind], labels_shape)
            return (
                _get_features_without_labels(
                    record, label_col_ind, features_shape
                ),
                labels,
            )
        {{end}}
        return tf.reshape(record, features_shape)

    dataset = dataset.map(_parse_data)

    {{if .IsTraining}}
    if mode != Mode.PREDICTION and "{{.TrainClause.EnableShuffle}}" == "true":
        dataset = dataset.shuffle(buffer_size={{.TrainClause.ShuffleBufferSize}})
    {{end}}

    return dataset


def eval_metrics_fn(predictions, labels):
    return {
        "dummy_metric": tf.reduce_sum(
            tf.reduce_mean(tf.reshape(predictions, [-1])) - labels
        )
    }


class PredictionOutputsProcessor(BasePredictionOutputsProcessor):
    def __init__(self):
        if all(
            k in os.environ
            for k in (
                "MAXCOMPUTE_PROJECT",
                "MAXCOMPUTE_AK",
                "MAXCOMPUTE_SK",
            )
        ):
            self.odps_writer = ODPSWriter(
                os.environ["MAXCOMPUTE_PROJECT"],
                os.environ["MAXCOMPUTE_AK"],
                os.environ["MAXCOMPUTE_SK"],
                os.environ.get("MAXCOMPUTE_ENDPOINT", None),
                table="{{.PredictOutputTable}}",
                columns=["pred_" + str(i) for i in range({{.OutputShape}})],
                column_types=["double" for _ in range({{.OutputShape}})],
            )
        else:
            self.odps_writer = None

    def process(self, predictions, worker_id):
        if self.odps_writer:
            self.odps_writer.from_iterator(
                iter(predictions.numpy().tolist()), worker_id
            )
        else:
            logger.info(predictions.numpy())
`
