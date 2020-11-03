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

import base64
import os

import matplotlib
import numpy as np
import pandas as pd
import six
import tensorflow as tf
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model.model import Model
from runtime.tensorflow import is_tf_estimator
from runtime.tensorflow.explain import explain_boosted_trees, explain_dnns
from runtime.tensorflow.import_model import import_model
from runtime.tensorflow.input_fn import input_fn
from runtime.tensorflow.keras_with_feature_column_input import \
    init_model_with_feature_column
from runtime.tensorflow.load_model import (load_keras_model_weights,
                                           pop_optimizer_and_loss)


def print_image_as_base64_html(file_path):
    with open(file_path, 'rb') as f:
        img = f.read()

    img = base64.b64encode(img)
    if six.PY3:
        img = img.decode('utf-8')
    img = "<div align='center'><img src='data:image/png;base64,%s' /></div>" \
          % img
    print(img)


def explain_step(datasource,
                 select,
                 explainer,
                 model_params,
                 result_table,
                 model,
                 pai_table=None,
                 oss_dest=None,
                 oss_ak=None,
                 oss_sk=None,
                 oss_endpoint=None,
                 oss_bucket_name=None):
    """
    Do explanation to a trained TensorFlow model.

    Args:
        datasource (str): the database connection string.
        select (str): the input data to predict.
        explainer (str): the explainer to explain the model.
                         Not used in TensorFlow models.
        model_params (dict): the parameters for evaluation.
        result_table (str): the output data table.
        model (Model|str): the model object or where to load the model.

    Returns:
        None.
    """
    if isinstance(model, six.string_types):
        model = Model.load_from_db(datasource, model)
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)

    plot_type = model_params.get("summary.plot_type", "bar")

    train_attributes = model.get_meta("attributes")
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

    label_name = model_params.get("label_col", train_label_desc.name)
    train_label_desc.name = label_name
    label_meta = train_label_desc.to_dict(dtype_to_string=True)

    if pai_table:
        assert oss_dest, "oss_dest must be given when submit to PAI"
    else:
        assert oss_dest is None

    if os.environ.get('DISPLAY', '') == '':
        print('no display found. Using non-interactive Agg backend')
        matplotlib.use('Agg')

    _explain(datasource=datasource,
             estimator_string=estimator_string,
             select=select,
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_metas=feature_metas,
             label_meta=label_meta,
             model_params=train_attributes,
             save=save,
             pai_table=pai_table,
             plot_type=plot_type,
             result_table=result_table,
             oss_dest=oss_dest,
             oss_ak=oss_ak,
             oss_sk=oss_sk,
             oss_endpoint=oss_endpoint,
             oss_bucket_name=oss_bucket_name)

    print_image_as_base64_html('summary.png')


def _explain(datasource,
             estimator_string,
             select,
             feature_columns,
             feature_column_names,
             feature_metas={},
             label_meta={},
             model_params={},
             save="",
             pai_table="",
             plot_type='bar',
             result_table="",
             oss_dest=None,
             oss_ak=None,
             oss_sk=None,
             oss_endpoint=None,
             oss_bucket_name=None):
    estimator_cls = import_model(estimator_string)
    if is_tf_estimator(estimator_cls):
        model_params['model_dir'] = save
    model_params.update(feature_columns)
    pop_optimizer_and_loss(model_params)

    is_pai = True if pai_table else False

    def _input_fn():
        dataset = input_fn(select,
                           datasource,
                           feature_column_names,
                           feature_metas,
                           label_meta,
                           is_pai=is_pai,
                           pai_table=pai_table)
        return dataset.batch(1).cache()

    estimator = init_model_with_feature_column(estimator_cls, model_params)
    if not is_tf_estimator(estimator_cls):
        load_keras_model_weights(estimator, save)

    if result_table:
        if is_pai:
            conn = PaiIOConnection.from_table(result_table)
        else:
            conn = db.connect_with_data_source(datasource)
    else:
        conn = None

    if estimator_cls in (tf.estimator.BoostedTreesClassifier,
                         tf.estimator.BoostedTreesRegressor):
        explain_boosted_trees(datasource, estimator, _input_fn, plot_type,
                              result_table, feature_column_names, conn,
                              oss_dest, oss_ak, oss_sk, oss_endpoint,
                              oss_bucket_name)
    else:
        shap_dataset = pd.DataFrame(columns=feature_column_names)
        for i, (features, label) in enumerate(_input_fn()):
            shap_dataset.loc[i] = [
                item.numpy()[0][0] for item in features.values()
            ]
        explain_dnns(datasource, estimator, shap_dataset, plot_type,
                     result_table, feature_column_names, conn, oss_dest,
                     oss_ak, oss_sk, oss_endpoint, oss_bucket_name)

    if conn is not None:
        conn.close()
