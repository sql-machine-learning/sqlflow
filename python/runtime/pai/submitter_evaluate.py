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

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.model import EstimatorType
from runtime.pai import cluster_conf, pai_model, table_ops
from runtime.pai.create_result_table import create_evaluate_result_table
from runtime.pai.get_pai_tf_cmd import (ENTRY_FILE, JOB_ARCHIVE_FILE,
                                        PARAMS_FILE, get_pai_tf_cmd)
from runtime.pai.prepare_archive import prepare_archive
from runtime.pai.submit_pai_task import submit_pai_task


def get_evaluate_metrics(model_type, model_attrs):
    """Get evaluate metrics from model attributes or return default

    Args:
        mode_type: type of the model, see runtime.model.EstimatorType
        model_attrs: model attributs passed by WITH clause

    Returns:
        An array of metrics names
    """
    metrics = []
    met_conf = model_attrs.get("validation.metrics") or model_attrs.get(
        "validationMetrics")
    if met_conf:
        [
            metrics.append(m) for m in met_conf.split(",")
            if m and m not in metrics
        ]
    # add default if no extra metrics is provided
    if len(metrics) == 0:
        if model_type == EstimatorType.XGBOOST:
            metrics.append("accuracy_score")
        elif model_type == EstimatorType.TENSORFLOW:
            metrics.append("Accuracy")
        else:
            raise SQLFlowDiagnostic("No metrics is provided.")
    return metrics


def submit_pai_evaluate(datasource,
                        original_sql,
                        select,
                        model_name,
                        model_params,
                        result_table,
                        user=""):
    """Submit a PAI evaluation task

    Args:
        datasource: string
            Like: maxcompute://ak:sk@domain.com/api?
                  curr_project=test_ci&scheme=http
        original_sql: string
            Original "TO PREDICT" statement.
        select: string
            SQL statement to get prediction data set.
        model_name: string
            Model to load and do prediction.
        model_params: dict
            Params for training, crossponding to WITH clause.
        result_table: string
            The table name to save prediction result.
        user: string
            A string to identify the user, used to load model from the user's
            directory.
    """

    params = dict(locals())
    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")

    project = table_ops.get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)
    params["result_table"] = result_table

    oss_model_path = pai_model.get_oss_model_save_path(datasource,
                                                       model_name,
                                                       user=user)
    params["oss_model_path"] = oss_model_path

    model_type, estimator = pai_model.get_oss_saved_model_type_and_estimator(
        oss_model_path, project)
    if model_type == EstimatorType.PAIML:
        raise SQLFlowDiagnostic("PAI model evaluation is not supported yet.")

    data_table = table_ops.create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    metrics = get_evaluate_metrics(model_type, model_params)
    params["metrics"] = metrics
    create_evaluate_result_table(datasource, result_table, metrics)

    conf = cluster_conf.get_cluster_config(model_params)

    if model_type == EstimatorType.XGBOOST:
        params["entry_type"] = "evaluate_xgb"
    else:
        params["entry_type"] = "evaluate_tf"
    prepare_archive(cwd, estimator, oss_model_path, params)
    cmd = get_pai_tf_cmd(conf, "file://" + os.path.join(cwd, JOB_ARCHIVE_FILE),
                         "file://" + os.path.join(cwd, PARAMS_FILE),
                         ENTRY_FILE, model_name, oss_model_path, data_table,
                         "", result_table, project)
    submit_pai_task(cmd, datasource)
    table_ops.drop_tables([data_table], datasource)
