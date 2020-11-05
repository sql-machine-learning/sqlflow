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
import six
import xgboost as xgb
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType, Model, oss
from runtime.pai.pai_distributed import define_tf_flags
from runtime.xgboost.dataset import DMATRIX_FILE_SEP, xgb_dataset
from runtime.xgboost.feature_column import ComposedColumnTransformer

FLAGS = define_tf_flags()


def predict(datasource,
            select,
            result_table,
            result_column_names,
            train_label_idx,
            model,
            extra_result_cols=[],
            pai_table="",
            oss_model_path=""):
    """TBD
    """
    is_pai = True if pai_table != "" else False
    if is_pai:
        # FIXME(typhoonzero): load metas from db instead.
        oss.load_file(oss_model_path, "my_model")
        (_, model_params, _, feature_metas, feature_column_names, _,
         fc_map_ir) = oss.load_metas(oss_model_path, "xgboost_model_desc")
    else:
        if isinstance(model, six.string_types):
            model = Model.load_from_db(datasource, model)
        else:
            assert isinstance(
                model, Model), "not supported model type %s" % type(model)

        model_params = model.get_meta("attributes")
        fc_map_ir = model.get_meta("features")

    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.XGBOOST)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])

    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    bst = xgb.Booster()
    bst.load_model("my_model")
    conn = db.connect_with_data_source(datasource)

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
