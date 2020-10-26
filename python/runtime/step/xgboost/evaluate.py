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
import sklearn.metrics
import xgboost as xgb
from runtime import db
from runtime.feature.compile import compile_ir_feature_columns
from runtime.feature.derivation import get_ordered_field_descs
from runtime.feature.field_desc import DataType
from runtime.model import EstimatorType, oss
from runtime.model.model import Model
from runtime.pai.pai_distributed import define_tf_flags
from runtime.step.create_result_table import create_evaluate_table
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
             pai_table="",
             oss_model_path=""):
    """TBD
    """
    if model_params is None:
        model_params = {}
    validation_metrics = model_params.get("validation.metrics",
                                          "accuracy_score")
    validation_metrics = [m.strip() for m in validation_metrics.split(",")]

    is_pai = True if pai_table != "" else False
    if is_pai:
        assert (oss_model_path != "")
        # NOTE(typhoonzero): the xgboost model file "my_model" is hard coded
        # in xgboost/train.py
        oss.load_file(oss_model_path, "my_model")
        (estimator, model_params, train_params, feature_metas,
         feature_column_names, train_label_desc,
         fc_map_ir) = oss.load_metas(oss_model_path, "xgboost_model_desc")
    else:
        if isinstance(model, six.string_types):
            model = Model.load_from_db(datasource, model)
        else:
            assert isinstance(
                model, Model), "not supported model type %s" % type(model)

        model_params = model.get_meta("attributes")
        fc_map_ir = model.get_meta("features")
        train_label_desc = model.get_meta("label").get_field_desc()[0]

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

    bst = xgb.Booster()
    bst.load_model("my_model")
    conn = db.connect_with_data_source(datasource)

    result_column_names = create_evaluate_table(conn, result_table,
                                                validation_metrics)

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
            transform_fn=transform_fn)

        for i, pred_dmatrix in enumerate(dpred):
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
