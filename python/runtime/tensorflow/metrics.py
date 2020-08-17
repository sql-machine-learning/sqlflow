# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import tensorflow as tf

metric_names_use_class_id = [
    "Accuracy", "Precision", "Recall", "TruePositives", "TrueNegatives",
    "FalsePositives", "FalseNegatives"
]
metric_names_use_probabilities = [
    "BinaryAccuracy", "CategoricalAccuracy", "TopKCategoricalAccuracy"
]
metric_names_use_predictions = [
    "MeanAbsoluteError", "MeanAbsolutePercentageError", "MeanSquaredError",
    "RootMeanSquaredError"
]
supported_metrics = metric_names_use_class_id \
                    + metric_names_use_probabilities \
                    + metric_names_use_predictions
supported_metrics += ["AUC"]


def check_supported(metrics):
    for mn in metrics:
        if mn not in supported_metrics:
            raise ValueError(
                "metric name not supported %s, supported metrics: %s" %
                (mn, supported_metrics))


def get_tf_metrics(metrics):
    check_supported(metrics)

    def tf_metrics_func(labels, predictions):
        metric_dict = {}
        for mn in metrics:
            if mn == "AUC":
                metric = tf.keras.metrics.AUC(num_thresholds=2000)
                metric.update_state(y_true=[labels],
                                    y_pred=predictions["logistic"])
            else:
                metric = eval("tf.keras.metrics.%s()" % mn)
                if mn in metric_names_use_class_id:
                    metric.update_state(y_true=[labels],
                                        y_pred=predictions["class_ids"])
                elif mn in metric_names_use_probabilities:
                    metric.update_state(y_true=[labels],
                                        y_pred=predictions["probabilities"])
                elif mn in metric_names_use_predictions:
                    metric.update_state(y_true=[labels],
                                        y_pred=predictions["predictions"])
            metric_dict[mn] = metric
        return metric_dict

    return tf_metrics_func


def get_keras_metrics(metrics):
    check_supported(metrics)
    m = []
    for mn in metrics:
        if mn == "AUC":
            m.append(tf.keras.metrics.AUC(num_thresholds=2000))
        else:
            m.append(eval("tf.keras.metrics.%s()" % mn))
    return m
