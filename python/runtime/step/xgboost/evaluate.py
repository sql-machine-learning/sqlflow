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
import six
import sklearn.metrics

import runtime.temp_file as temp_file
import xgboost as xgb
from runtime import db
from runtime.dbapi.paiio import PaiIOConnection
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model import EstimatorType
from runtime.model.model import Model
from runtime.pai.pai_distributed import define_tf_flags
from runtime.step.xgboost.predict import _calc_predict_result
from runtime.xgboost.dataset import xgb_dataset
# TODO(typhoonzero): remove runtime.xgboost
from runtime.xgboost.feature_column import ComposedColumnTransformer

FLAGS = define_tf_flags()

SKLEARN_METRICS = [
    'accuracy_score',
    'average_precision_score',
    'balanced_accuracy_score',
    'brier_score_loss',
    'cohen_kappa_score',
    'explained_variance_score',
    'f1_score',
    'fbeta_score',
    'hamming_loss',
    'hinge_loss',
    'log_loss',
    'mean_absolute_error',
    'mean_squared_error',
    'mean_squared_log_error',
    'median_absolute_error',
    'precision_score',
    'r2_score',
    'recall_score',
    'roc_auc_score',
    'zero_one_loss',
]


def evaluate(datasource,
             select,
             result_table,
             model,
             label_name=None,
             model_params=None,
             result_column_names=[],
             pai_table=None):
    """TBD
    """
    if model_params is None:
        model_params = {}
    validation_metrics = model_params.get("validation.metrics",
                                          "accuracy_score")
    validation_metrics = [m.strip() for m in validation_metrics.split(",")]

    bst = xgb.Booster()
    if isinstance(model, six.string_types):
        with temp_file.TemporaryDirectory(as_cwd=True):
            model = Model.load_from_db(datasource, model)
            bst.load_model("my_model")
    else:
        assert isinstance(model,
                          Model), "not supported model type %s" % type(model)
        bst.load_model("my_model")

    model_params = model.get_meta("attributes")
    fc_map_ir = model.get_meta("features")
    train_label = model.get_meta("label")
    train_label_desc = train_label.get_field_desc()[0]

    if label_name:
        train_label_desc.name = label_name

    feature_columns = compile_ir_feature_columns(fc_map_ir,
                                                 EstimatorType.XGBOOST)
    field_descs = get_ordered_field_descs(fc_map_ir)
    feature_column_names = [fd.name for fd in field_descs]
    feature_metas = dict([(fd.name, fd.to_dict(dtype_to_string=True))
                          for fd in field_descs])
    transform_fn = ComposedColumnTransformer(
        feature_column_names, *feature_columns["feature_columns"])

    is_pai = True if pai_table else False
    if is_pai:
        conn = PaiIOConnection.from_table(pai_table)
    else:
        conn = db.connect_with_data_source(datasource)

    with temp_file.TemporaryDirectory() as tmp_dir_name:
        pred_fn = os.path.join(tmp_dir_name, "predict.txt")

        dpred = xgb_dataset(
            datasource=datasource,
            fn=pred_fn,
            dataset_sql=select,
            feature_metas=feature_metas,
            feature_column_names=feature_column_names,
            label_meta=train_label_desc.to_dict(dtype_to_string=True),
            cache=True,
            batch_size=10000,
            transform_fn=transform_fn,
            is_pai=is_pai,
            pai_table=pai_table,
            pai_single_file=True,
            feature_column_code=fc_map_ir)

        for i, pred_dmatrix in enumerate(dpred):
            if is_pai:
                feature_file_name = pred_fn
            else:
                feature_file_name = pred_fn + "_%d" % i
            preds = _calc_predict_result(bst, pred_dmatrix, model_params)
            _store_evaluate_result(preds, feature_file_name, train_label_desc,
                                   result_table, result_column_names,
                                   validation_metrics, conn)

    conn.close()


def _store_evaluate_result(preds, feature_file_name, label_desc, result_table,
                           result_column_names, validation_metrics, conn):
    """
    Save the evaluation result in the table.

    Args:
        preds: the prediction result.
        feature_file_name (str): the file path where the feature dumps.
        label_desc (FieldDesc): the label FieldDesc object.
        result_table (str): the result table name.
        result_column_names (list[str]): the result column names.
        validation_metrics (list[str]): the evaluation metric names.
        conn: the database connection object.

    Returns:
        None.
    """
    y_test = []
    with open(feature_file_name, 'r') as f:
        for line in f.readlines():
            row = [i for i in line.strip().split("\t")]
            # DMatrix store label in the first column
            if label_desc.dtype == DataType.INT64:
                y_test.append(int(row[0]))
            elif label_desc.dtype == DataType.FLOAT32:
                y_test.append(float(row[0]))
            else:
                raise TypeError("unsupported data type {}".format(
                    label_desc.dtype))

    y_test = np.array(y_test)

    evaluate_results = dict()
    for metric_name in validation_metrics:
        metric_name = metric_name.strip()
        if metric_name not in SKLEARN_METRICS:
            raise ValueError("unsupported metrics %s" % metric_name)
        metric_func = getattr(sklearn.metrics, metric_name)
        metric_value = metric_func(y_test, preds)
        evaluate_results[metric_name] = metric_value

    # write evaluation result to result table
    with db.buffered_db_writer(conn, result_table, result_column_names) as w:
        row = ["0.0"]
        for mn in validation_metrics:
            row.append(str(evaluate_results[mn]))
        w.write(row)
