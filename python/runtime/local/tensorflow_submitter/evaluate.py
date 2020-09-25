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

import six
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.local.create_result_table import create_evaluate_table
from runtime.model.model import Model
from runtime.tensorflow.evaluate import evaluate as _evaluate


def evaluate(datasource,
             select,
             result_table,
             model,
             pred_label_name=None,
             model_params=None):
    """
    Do evaluation to a trained TensorFlow model.

    Args:
        datasource (str): the database connection string.
        select (str): the input data to predict.
        result_table (str): the output data table.
        model (Model|str): the model object or where to load the model.
        pred_label_name (str): the label column name.
        model_params (dict): the parameters for evaluation.

    Returns:
        None.
    """
    if isinstance(model, six.string_types):
        model = Model.load_from_db(datasource, model)
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)

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
    train_label_desc.name = pred_label_name
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
              verbose=verbose)
