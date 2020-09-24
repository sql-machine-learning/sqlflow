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

import numpy as np
import six
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model.model import Model
from runtime.tensorflow.explain import explain as _explain


def explain(datasource, select, explainer, model_params, result_table, model):
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

    if result_table:
        conn = db.connect_with_data_source(datasource)
        if estimator_string.startswith("BoostedTrees"):
            column_defs = [
                "feature %s" %
                DataType.to_db_field_type(conn.driver, DataType.STRING),
                "dfc %s" %
                DataType.to_db_field_type(conn.driver, DataType.FLOAT32),
                "gain %s" %
                DataType.to_db_field_type(conn.driver, DataType.FLOAT32),
            ]
        else:
            selected_cols = db.selected_cols(conn, select)
            if label_name in selected_cols:
                selected_cols.remove(label_name)

            name_to_shape = dict([(fd.name, fd.shape) for fd in field_descs])
            column_defs = []
            float_field_type = DataType.to_db_field_type(
                conn.driver, DataType.FLOAT32)
            for name in selected_cols:
                shape = name_to_shape.get(name, None)
                if shape is None:
                    raise ValueError("cannot find column %s" % name)

                size = int(np.prod(shape))
                if size == 1:
                    column_def = "%s %s" % (name, float_field_type)
                    column_defs.append(column_def)
                else:
                    for i in six.moves.range(size):
                        column_def = "%s_%d %s" % (name, i, float_field_type)
                        column_defs.append(column_def)

        drop_sql = "DROP TABLE IF EXISTS %s;" % result_table
        create_sql = "CREATE TABLE %s (%s);" % (result_table,
                                                ",".join(column_defs))
        conn.execute(drop_sql)
        conn.execute(create_sql)
        conn.close()

    _explain(datasource=datasource,
             estimator_string=estimator_string,
             select=select,
             feature_columns=feature_columns,
             feature_column_names=feature_column_names,
             feature_metas=feature_metas,
             label_meta=label_meta,
             model_params=train_attributes,
             save=save,
             plot_type=plot_type,
             result_table=result_table)

    with open('summary.png', 'rb') as f:
        img = f.read()

    img = base64.b64encode(img)
    if six.PY3:
        img = img.decode('utf-8')
    img = "<div align='center'><img src='data:image/png;base64,%s' /></div>" \
          % img
    print(img)
