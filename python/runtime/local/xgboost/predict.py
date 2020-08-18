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

import os
import tempfile

import numpy as np
import runtime.temp_file as temp_file
import runtime.xgboost as xgboost_extended
import xgboost as xgb
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model.model import Model
from runtime.xgboost.dataset import xgb_dataset


def pred(datasource, select, result_table, pred_label_name, load):
    """
    Do prediction using a trained model.

    Args:
        datasource (str): the database connection string.
        select (str): the input data to predict.
        result_table (str): the output data table.
        pred_label_name (str): the output label name to predict.
        load (str): where the trained model stores.

    Returns:
        None.
    """
    model = Model.load_from_db(datasource, load)
    model_params = model.get_meta("attributes")
    train_fc_map = model.get_meta("features")
    train_label_desc = model.get_meta("label").get_field_desc()[0]

    field_descs = get_ordered_field_descs(train_fc_map)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict()) for fd in field_descs])

    # NOTE: in the current implementation, we are generating a transform_fn
    # from the COLUMN clause. The transform_fn is executed during the process
    # of dumping the original data into DMatrix SVM file.
    compiled_fc = compile_ir_feature_columns(train_fc_map, model.get_type())
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *compiled_fc["feature_columns"])

    bst = xgb.Booster()
    bst.load_model("my_model")

    conn = db.connect_with_data_source(datasource)
    result_column_names, train_label_idx = _create_predict_table(
        conn, select, result_table, train_label_desc, pred_label_name)

    with temp_file.TemporaryDirectory() as tmp_dir_name:
        pred_fn = os.path.join(tmp_dir_name, "predict.txt")
        raw_data_dir = os.path.join(tmp_dir_name, "predict_raw_dir")

        dpred = xgb_dataset(
            datasource=datasource,
            fn=pred_fn,
            dataset_sql=select,
            feature_metas=feature_metas,
            feature_column_names=feature_column_names,
            label_meta=None,
            cache=True,
            batch_size=10000,
            transform_fn=transform_fn,
            raw_data_dir=raw_data_dir)  # NOTE: default to use external memory

        print("Start predicting XGBoost model...")
        for idx, pred_dmatrix in enumerate(dpred):
            feature_file_name = os.path.join(
                tmp_dir_name, "predict_raw_dir/predict.txt_%d" % idx)
            _predict_and_store_result(bst, pred_dmatrix, model_params,
                                      result_table, result_column_names,
                                      train_label_idx, feature_file_name, conn)
        print("Done predicting. Predict table : %s" % result_table)

    conn.close()


def _predict_and_store_result(bst, dpred, model_params, result_table,
                              result_column_names, train_label_idx,
                              feature_file_name, conn):
    """
    Do prediction and save the prediction result in the table.

    Args:
        bst: the XGBoost booster object.
        dpred: the XGBoost DMatrix input data to predict.
        model_params (dict): the XGBoost model parameters.
        result_table (str): the result table name.
        result_column_names (list[str]): the result column names.
        train_label_idx (int): the index where the trained label is inside
            result_column_names.
        feature_file_name (str): the file path where the feature dumps.
        conn: the database connection object.

    Returns:
        None.
    """
    preds = bst.predict(dpred)

    # TODO(yancey1989): should save train_params and model_params
    # not only on PAI submitter
    # TODO(yancey1989): output the original result for various
    # objective function.
    objective = model_params.get("objective", "")
    if objective.startswith("binary:"):
        preds = (preds > 0.5).astype(np.int64)
    elif objective.startswith("multi:") and len(preds) == 2:
        preds = np.argmax(np.array(preds), axis=1)

    with db.buffered_db_writer(conn, result_table, result_column_names,
                               100) as w:
        with open(feature_file_name, "r") as feature_file_read:
            line_no = 0
            for line in feature_file_read.readlines():
                if not line:
                    break

                row = [
                    item for i, item in enumerate(line.strip().split("/"))
                    if i != train_label_idx
                ]
                row.append(str(preds[line_no]))
                w.write(row)
                line_no += 1


def _create_predict_table(conn, select, result_table, train_label_desc,
                          pred_label_name):
    """
    Create the result prediction table.

    Args:
        conn: the database connection object.
        select (str): the input data to predict.
        result_table (str): the output data table.
        train_label_desc (FieldDesc): the FieldDesc of the trained label.
        pred_label_name (str): the output label name to predict.

    Returns:
        A tuple of (result_column_names, train_label_index).
    """
    name_and_types = db.selected_columns_and_types(conn, select)
    train_label_index = -1
    for i, (name, _) in enumerate(name_and_types):
        if name == train_label_desc.name:
            train_label_index = i
            break

    if train_label_index >= 0:
        del name_and_types[train_label_index]

    column_strs = []
    for name, typ in name_and_types:
        column_strs.append("%s %s" %
                           (name, db.to_db_field_type(conn.driver, typ)))

    train_label_field_type = DataType.to_db_field_type(conn.driver,
                                                       train_label_desc.dtype)
    column_strs.append("%s %s" % (pred_label_name, train_label_field_type))

    drop_sql = "DROP TABLE IF EXISTS %s;" % result_table
    create_sql = "CREATE TABLE %s (%s);" % (result_table,
                                            ",".join(column_strs))
    conn.execute(drop_sql)
    conn.execute(create_sql)
    result_column_names = [item[0] for item in name_and_types]
    result_column_names.append(pred_label_name)
    return result_column_names, train_label_index
