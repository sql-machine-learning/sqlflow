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

    precision = tf.keras.metrics.Precision(name="precision")
    precision.update_state(y_true=labels, y_pred=predictions['class_ids'])

    # recall = tf.keras.metrics.Recall(name="recall")
    # recall.update_state(y_true=labels, y_pred=predictions['all_class_ids'])
    
    # auc = tf.keras.metrics.AUC(name="auc")
    # auc.update_state(y_true=labels, y_pred=predictions['probabilities'])

    return {"accuracy": accuracy, "precision": precision}#, "recall": recall, "auc": auc} 

def keras_classification_metrics():
    return [tf.keras.metrics.Accuracy(), tf.keras.metrics.Precision(),
        tf.keras.metrics.Recall(), tf.keras.metrics.AUC()]

def tf_regression_metrics(labels, predictions):
    rmse = tf.keras.metrics.RootMeanSquaredError(name="rmse")
    rmse.update_state(y_true=labels, y_pred=predictions['logits'])

    mae = tf.keras.metrics.MeanAbsoluteError(name="mae")
    mae.update_state(y_true=labels, y_pred=predictions['logits'])

    mape = tf.keras.metrics.MeanAbsolutePercentageError(name="mape")
    mape.update_state(y_true=labels, y_pred=predictions['logits'])

    return {"rmse": rmse, "mae": mae, "mape": mape}

def keras_regression_metrics():
    return [tf.keras.metrics.RootMeanSquaredError(),
        tf.keras.metrics.MeanAbsoluteError(),
        tf.keras.metrics.MeanAbsolutePercentageError()]