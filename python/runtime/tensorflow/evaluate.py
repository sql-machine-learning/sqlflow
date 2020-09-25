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

import sys

from runtime.db import buffered_db_writer, connect_with_data_source
from runtime.tensorflow import metrics
from runtime.tensorflow.get_tf_model_type import is_tf_estimator
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import (load_keras_model_weights,
                                           pop_optimizer_and_loss)
from runtime.tensorflow.set_log_level import set_log_level


def evaluate(datasource,
             estimator_string,
             select,
             result_table,
             feature_columns,
             feature_column_names,
             feature_metas={},
             label_meta={},
             model_params={},
             validation_metrics=["Accuracy"],
             save="",
             batch_size=1,
             validation_steps=None,
             verbose=0):
    estimator_cls = import_model(estimator_string)
    is_estimator = is_tf_estimator(estimator_cls)
    set_log_level(verbose, is_estimator)
    eval_dataset = get_dataset_fn(select,
                                  datasource,
                                  feature_column_names,
                                  feature_metas,
                                  label_meta,
                                  is_pai=False,
                                  pai_table="",
                                  batch_size=batch_size)

    model_params.update(feature_columns)
    pop_optimizer_and_loss(model_params)
    if is_estimator:
        model_params["model_dir"] = save
        estimator = estimator_cls(**model_params)
        result_metrics = estimator_evaluate(estimator, eval_dataset,
                                            validation_metrics)
    else:
        keras_model = init_model_with_feature_column(estimator_cls,
                                                     model_params)
        keras_model_pkg = sys.modules[estimator_cls.__module__]
        result_metrics = keras_evaluate(keras_model, eval_dataset, save,
                                        keras_model_pkg, validation_metrics)

    # write result metrics to a table
    conn = connect_with_data_source(datasource)
    if result_table:
        metric_name_list = ["loss"] + validation_metrics
        write_result_metrics(result_metrics, metric_name_list, result_table,
                             conn)
    conn.close()


def estimator_evaluate(estimator, eval_dataset, validation_metrics):
    result = estimator.evaluate(eval_dataset)
    avg_loss = result["average_loss"]
    result_metrics = dict()
    result_metrics["loss"] = float(avg_loss)
    for m in validation_metrics:
        val = float(result.get(m.lower()))
        if val:
            result_metrics[m] = val
        else:
            # NOTE: estimator automatically append metrics for the current
            # evaluation job, if user specified metrics not appear in
            # estimator's result dict, fill None.
            print(
                "specified metric %s not calculated by estimator, fill empty "
                "value." % m)
            result_metrics[m] = None

    return result_metrics


def keras_evaluate(keras_model, eval_dataset_fn, save, keras_model_pkg,
                   validation_metrics):
    model_metrics = []
    if hasattr(keras_model_pkg, "eval_metrics_fn"):
        metrics_functions = keras_model_pkg.eval_metrics_fn()
        for key, func in metrics_functions.items():
            func.__name__ = key
            model_metrics.append(func)
    # use WITH specified metrics if it's not default.
    if validation_metrics != ["Accuracy"]:
        keras_metrics = metrics.get_keras_metrics(validation_metrics)
    else:
        if len(model_metrics) > 0:
            keras_metrics = model_metrics
        else:
            # default
            keras_metrics = metrics.get_keras_metrics(["Accuracy"])
    has_custom_evaluate_func = hasattr(keras_model, 'sqlflow_evaluate_loop')

    if not has_custom_evaluate_func:
        # compile the model with default arguments only for evaluation
        # (run forward only).
        keras_model.compile(loss=keras_model_pkg.loss, metrics=keras_metrics)

    eval_dataset = eval_dataset_fn()

    def get_features(sample, label):
        return sample

    eval_dataset_x = eval_dataset.map(get_features)

    if has_custom_evaluate_func:
        result = keras_model.sqlflow_evaluate_loop(eval_dataset,
                                                   validation_metrics)
    else:
        one_batch = next(iter(eval_dataset_x))
        # NOTE: must run predict one batch to initialize parameters
        # see: https://www.tensorflow.org/alpha/guide/keras/saving_and_serializing#saving_subclassed_models # noqa: E501
        keras_model.predict_on_batch(one_batch)
        load_keras_model_weights(keras_model, save)
        result = keras_model.evaluate(eval_dataset)

    assert (len(result) == len(validation_metrics) + 1)
    result_metrics = dict()
    for idx, m in enumerate(["loss"] + validation_metrics):
        result_metrics[m] = float(result[idx])
    return result_metrics


def write_result_metrics(result_metrics, metric_name_list, result_table, conn):
    # NOTE: assume that the result table is already created with columns:
    # loss | metric_names ...
    column_names = metric_name_list
    with buffered_db_writer(conn, result_table, column_names, 100) as w:
        row = []
        for key in metric_name_list:
            row.append(result_metrics[key])
        w.write(row)
