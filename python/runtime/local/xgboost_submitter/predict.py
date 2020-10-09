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

import numpy as np
import runtime.temp_file as temp_file
import runtime.xgboost as xgboost_extended
import six
import xgboost as xgb
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.local.create_result_table import create_predict_table
from runtime.model.model import Model
from runtime.xgboost.dataset import DMATRIX_FILE_SEP, xgb_dataset


def pred(datasource, select, result_table, pred_label_name, model):
    """
    Do prediction using a trained model.

    Args:
        datasource (str): the database connection string.
        select (str): the input data to predict.
        result_table (str): the output data table.
        pred_label_name (str): the output label name to predict.
        model (Model|str): the model object or where to load the model.

    Returns:
        None.
    """
    if isinstance(model, six.string_types):
        model = Model.load_from_db(datasource, model)
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)

    model_params = model.get_meta("attributes")
    train_fc_map = model.get_meta("features")
    train_label_desc = model.get_meta("label").get_field_desc()[0]

    field_descs = get_ordered_field_descs(train_fc_map)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])

    # NOTE: in the current implementation, we are generating a transform_fn
    # from the COLUMN clause. The transform_fn is executed during the process
    # of dumping the original data into DMatrix SVM file.
    compiled_fc = compile_ir_feature_columns(train_fc_map, model.get_type())
    transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(
        feature_column_names, *compiled_fc["feature_columns"])

    bst = xgb.Booster()
    bst.load_model("my_model")

    conn = db.connect_with_data_source(datasource)
    result_column_names, train_label_idx = create_predict_table(
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
            preds = _calc_predict_result(bst, pred_dmatrix, model_params)
            _store_predict_result(preds, result_table, result_column_names,
                                  train_label_idx, feature_file_name, conn)
        print("Done predicting. Predict table : %s" % result_table)

    conn.close()


def _calc_predict_result(bst, dpred, model_params):
    """
    Calculate the prediction result.

    Args:
        bst: the XGBoost booster object.
        dpred: the XGBoost DMatrix input data to predict.
        model_params (dict): the XGBoost model parameters.

    Returns:
        The prediction result.
    """
    preds = bst.predict(dpred)
    preds = np.array(preds)

    # TODO(yancey1989): should save train_params and model_params
    # not only on PAI submitter
    # TODO(yancey1989): output the original result for various
    # objective function.
    obj = model_params.get("objective", "")
    # binary:hinge output class labels
    if obj.startswith("binary:logistic"):
        preds = (preds > 0.5).astype(int)
    # multi:softmax output class labels
    elif obj.startswith("multi:softprob"):
        preds = np.argmax(np.array(preds), axis=1)
    # TODO(typhoonzero): deal with binary:logitraw when needed.

    return preds


def _store_predict_result(preds, result_table, result_column_names,
                          train_label_idx, feature_file_name, conn):
    """
    Save the prediction result in the table.

    Args:
        preds: the prediction result to save.
        result_table (str): the result table name.
        result_column_names (list[str]): the result column names.
        train_label_idx (int): the index where the trained label is inside
            result_column_names.
        feature_file_name (str): the file path where the feature dumps.
        conn: the database connection object.

    Returns:
        None.
    """
    with db.buffered_db_writer(conn, result_table, result_column_names) as w:
        with open(feature_file_name, "r") as feature_file_read:
            line_no = 0
            for line in feature_file_read.readlines():
                if not line:
                    break

                row = [
                    item for i, item in enumerate(line.strip().split(
                        DMATRIX_FILE_SEP)) if i != train_label_idx
                ]
                row.append(str(preds[line_no]))
                w.write(row)
                line_no += 1
