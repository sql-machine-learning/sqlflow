# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

def tf_classification_metrics(labels, predictions):
    print(predictions)
    accuracy = tf.keras.metrics.Accuracy(name="accuracy")
    accuracy.update_state(y_true=labels, y_pred=predictions['class_ids'])
    
    auc = tf.keras.metrics.AUC(name="auc", num_thresholds=2000)
    auc.update_state(y_true=labels, y_pred=predictions['probabilities'])

    # TODO(typhoonzero): precision & recall is defined for a specific class
    # since we may encounter multi-class classification here, we do not calculate
    # precision by default. We need find a way to determine whether
    # the job is binary classificaion.
    return {"accuracy": accuracy, "auc": auc}


def tf_regression_metrics(labels, predictions):
    rmse = tf.keras.metrics.RootMeanSquaredError(name="rmse")
    rmse.update_state(y_true=labels, y_pred=predictions['logits'])

    mae = tf.keras.metrics.MeanAbsoluteError(name="mae")
    mae.update_state(y_true=labels, y_pred=predictions['logits'])

    mape = tf.keras.metrics.MeanAbsolutePercentageError(name="mape")
    mape.update_state(y_true=labels, y_pred=predictions['logits'])

    return {"rmse": rmse, "mae": mae, "mape": mape}

metric_names_use_class_id = ["Accuracy", "Precision", "Recall", "TruePositives", "TrueNegatives", "FalsePositives", "FalseNegatives"]
metric_names_use_probabilities = ["BinaryAccuracy", "CategoricalAccuracy", "TopKCategoricalAccuracy", "AUC"]
metric_names_use_logits = ["MeanAbsoluteError", "MeanAbsolutePercentageError", "MeanSquaredError", "RootMeanSquaredError"]
supported_metrics = metric_names_use_class_id + metric_names_use_probabilities + metric_names_use_logits

def get_tf_metrics(metrics):
    def tf_metrics_func(labels, predictions):
        metric_dict = {}
        for mn in metrics:
            metric = eval("tf.keras.metrics.%s()" % mn)
            if mn in metric_names_use_class_id:
                metric.update_state(y_true=labels, y_pred=predictions["class_ids"])
            elif mn in metric_names_use_probabilities:
                metric.update_state(y_true=labels, y_pred=predictions["probabilities"])
            elif mn in metric_names_use_logits:
                metric.update_state(y_true=labels, y_pred=predictions["logits"])
            metric_dict[mn] = metric
        return metric_dict
    return tf_metrics_func

def get_keras_metrics(metrics):
    m = []
    for mn in metrics:
        m.append(eval("tf.keras.metrics.%s()" % mn))
    return m