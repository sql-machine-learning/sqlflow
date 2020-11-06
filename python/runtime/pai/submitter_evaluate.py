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

import runtime.temp_file as temp_file
from runtime import db
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.model import EstimatorType
from runtime.pai import cluster_conf, pai_model, table_ops
from runtime.pai.get_pai_tf_cmd import (ENTRY_FILE, JOB_ARCHIVE_FILE,
                                        PARAMS_FILE, get_pai_tf_cmd)
from runtime.pai.prepare_archive import prepare_archive
from runtime.pai.submit_pai_task import submit_pai_task
from runtime.pai_local.try_run import try_pai_local_run
from runtime.step.create_result_table import create_evaluate_table


def submit_pai_evaluate(datasource,
                        original_sql,
                        select,
                        model,
                        label_name,
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
        model: string
            Model to load and do prediction.
        label_name: string
            The label name to evaluate.
        model_params: dict
            Params for training, crossponding to WITH clause.
        result_table: string
            The table name to save prediction result.
        user: string
            A string to identify the user, used to load model from the user's
            directory.
    """

    params = dict(locals())
    project = table_ops.get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)
    params["result_table"] = result_table

    oss_model_path = pai_model.get_oss_model_save_path(datasource,
                                                       model,
                                                       user=user)

    model_type, estimator = pai_model.get_saved_model_type_and_estimator(
        datasource, model)
    if model_type == EstimatorType.PAIML:
        raise SQLFlowDiagnostic("PAI model evaluation is not supported yet.")

    if model_type == EstimatorType.XGBOOST:
        params["entry_type"] = "evaluate_xgb"
        validation_metrics = model_params.get("validation.metrics",
                                              "accuracy_score")
    else:
        params["entry_type"] = "evaluate_tf"
        validation_metrics = model_params.get("validation.metrics", "Accuracy")

    validation_metrics = [m.strip() for m in validation_metrics.split(",")]
    with db.connect_with_data_source(datasource) as conn:
        result_column_names = create_evaluate_table(conn, result_table,
                                                    validation_metrics)

    with table_ops.create_tmp_tables_guard(select, datasource) as data_table:
        params["pai_table"] = data_table
        params["result_column_names"] = result_column_names

        if try_pai_local_run(params, oss_model_path):
            return

        conf = cluster_conf.get_cluster_config(model_params)
        with temp_file.TemporaryDirectory(prefix="sqlflow", dir="/tmp") as cwd:
            prepare_archive(cwd, estimator, oss_model_path, params)
            cmd = get_pai_tf_cmd(
                conf, "file://" + os.path.join(cwd, JOB_ARCHIVE_FILE),
                "file://" + os.path.join(cwd, PARAMS_FILE), ENTRY_FILE, model,
                oss_model_path, data_table, "", result_table, project)
            submit_pai_task(cmd, datasource)
