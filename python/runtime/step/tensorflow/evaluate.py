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
# limitations under the License

import sys

import six
import tensorflow as tf
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model.model import Model
from runtime.pai.pai_distributed import define_tf_flags
from runtime.step.create_result_table import create_evaluate_table
from runtime.tensorflow import is_tf_estimator
from runtime.tensorflow.evaluate import (estimator_evaluate, keras_evaluate,
                                         write_result_metrics)
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import get_dataset_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import pop_optimizer_and_loss
from runtime.tensorflow.set_log_level import set_log_level

try:
    tf.enable_eager_execution()
except Exception as e:
    sys.stderr.write("warning: failed to enable_eager_execution: %s" % e)
    pass

FLAGS = define_tf_flags()


def evaluate_step(datasource,
                  select,
                  result_table,
                  model,
                  label_name,
                  model_params,
                  pai_table=None):
    if isinstance(model, six.string_types):
        model = Model.load_from_db(datasource, model)
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)

    if model_params is None:
        model_params = {}

    validation_metrics = model_params.get("validation.metrics", "Accuracy")
    validation_metrics = [m.strip() for m in validation_metrics.split(',')]
    validation_steps = model_params.get("validation.steps", None)
    batch_size = model_params.get("validation.batch_size", 1)
    verbose = model_params.get("validation.verbose", 0)

    conn = db.connect_with_data_source(datasource)
    create_evaluate_table(conn, result_table, validation_metrics)
    conn.close()

    model_params = model.get_meta("attributes")
    train_fc_map = model.get_meta("features")
    train_label_desc = model.get_meta("label").get_field_desc()[0]
    estimator_string = model.get_meta("class_name")
    save = "model_save"

    field_descs = get_ordered_field_descs(train_fc_map)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])
    feature_columns = compile_ir_feature_columns(train_fc_map,
                                                 model.get_type())
    train_label_desc.name = label_name
    label_meta = train_label_desc.to_dict(dtype_to_string=True)

    _evaluate(datasource=datasource,
              estimator_string=estimator_string,
              select=select,
              result_table=result_table,
              feature_columns=feature_columns,
              feature_column_names=feature_column_names,
              feature_metas=feature_metas,
              label_meta=label_meta,
              model_params=model_params,
              validation_metrics=validation_metrics,
              save=save,
              batch_size=batch_size,
              validation_steps=validation_steps,
              verbose=verbose,
              pai_table=pai_table)


def _evaluate(datasource,
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
              verbose=0,
              pai_table=""):
    estimator_cls = import_model(estimator_string)
    is_estimator = is_tf_estimator(estimator_cls)
    set_log_level(verbose, is_estimator)

    is_pai = True if pai_table else False
    eval_dataset = get_dataset_fn(select,
                                  datasource,
                                  feature_column_names,
                                  feature_metas,
                                  label_meta,
                                  is_pai=is_pai,
                                  pai_table=pai_table,
                                  batch_size=batch_size)

    model_params.update(feature_columns)
    pop_optimizer_and_loss(model_params)
    if is_estimator:
        with open("exported_path", "r") as fid:
            exported_path = str(fid.read())

        model_params["warm_start_from"] = exported_path
        estimator = estimator_cls(**model_params)
        result_metrics = estimator_evaluate(estimator, eval_dataset,
                                            validation_metrics)
    else:
        keras_model = init_model_with_feature_column(estimator_cls,
                                                     model_params)
        keras_model_pkg = sys.modules[estimator_cls.__module__]
        result_metrics = keras_evaluate(keras_model, eval_dataset, save,
                                        keras_model_pkg, validation_metrics)

    if result_table:
        metric_name_list = ["loss"] + validation_metrics
        if is_pai:
            conn = PaiIOConnection.from_table(result_table)
        else:
            conn = db.connect_with_data_source(datasource)
        write_result_metrics(result_metrics, metric_name_list, result_table,
                             conn)
        conn.close()
