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

from runtime import db
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.model import EstimatorType
from runtime.pai import cluster_conf, pai_model, table_ops
from runtime.pai.create_result_table import create_explain_result_table
from runtime.pai.get_pai_tf_cmd import (ENTRY_FILE, JOB_ARCHIVE_FILE,
                                        PARAMS_FILE, get_pai_tf_cmd)
from runtime.pai.prepare_archive import prepare_archive
from runtime.pai.submit_pai_task import submit_pai_task


def get_explain_random_forests_cmd(datasource, model_name, data_table,
                                   result_table, label_column):
    """Get PAI random forest explanation command

    Args:
        datasource: current datasoruce
        model_name: model name on PAI
        data_table: input data table name
        result_table: result table name
        label_column: name of the label column

    Returns:
        a PAI cmd to explain the data using given model
    """
    # NOTE(typhoonzero): for PAI random forests predicting, we can not load
    # the TrainStmt since the model saving is fully done by PAI. We directly
    # use the columns in SELECT statement for prediction, error will be
    # reported by PAI job if the columns not match.
    if not label_column:
        raise SQLFlowDiagnostic("must specify WITH label_column when using "
                                "pai random forest to explain models")

    conn = db.connect_with_data_source(datasource)
    # drop result table if exists
    conn.execute("DROP TABLE IF EXISTS %s;" % result_table)
    schema = db.get_table_schema(conn, data_table)
    fields = [f[0] for f in schema if f[0] != label_column]
    return ('''pai -name feature_importance -project algo_public '''
            '''-DmodelName="%s" -DinputTableName="%s"  '''
            '''-DoutputTableName="%s" -DlabelColName="%s" '''
            '''-DfeatureColNames="%s" ''') % (model_name, data_table,
                                              result_table, label_column,
                                              ",".join(fields))


def setup_explain_entry(params, model_type):
    """Setup PAI prediction entry function according to model type"""
    if model_type == EstimatorType.TENSORFLOW:
        params["entry_type"] = "explain_tf"
    elif model_type == EstimatorType.PAIML:
        params["entry_type"] = ""
    elif model_type == EstimatorType.XGBOOST:
        params["entry_type"] = "explain_xgb"
    else:
        raise SQLFlowDiagnostic("unsupported model type: %d" % model_type)


def get_pai_explain_cmd(datasource, project, oss_model_path, model_name,
                        data_table, result_table, model_type, model_params,
                        job_file, params_file, label_column, cwd):
    """Get command to submit explain task to PAI

    Args:
        datasource: current datasource
        project: current project
        oss_model_path: the place to load model
        model_name: model used to do prediction
        data_table: data table from which to load explain data
        result_table: table to store prediction result
        model_type: type of th model, see also get_oss_saved_model_type
        model_params: parameters specified by WITH clause
        job_file: tar file incldue code and libs to execute on PAI
        params_file: extra params file
        lable_column: name of the label
        cwd: current working dir

    Returns:
        The command to submit a PAI explain task
    """
    if model_type == EstimatorType.PAIML:
        cmd = get_explain_random_forests_cmd(datasource, model_name,
                                             data_table, result_table,
                                             label_column)
    else:
        conf = cluster_conf.get_cluster_config(model_params)
        cmd = get_pai_tf_cmd(conf,
                             "file://" + os.path.join(cwd, JOB_ARCHIVE_FILE),
                             "file://" + os.path.join(cwd, PARAMS_FILE),
                             ENTRY_FILE, model_name, oss_model_path,
                             data_table, "", result_table, project)
    return cmd


def submit_pai_explain(datasource,
                       original_sql,
                       select,
                       model_name,
                       model_params,
                       result_table,
                       explainer="TreeExplainer",
                       user=""):
    """This function pack need params and resource to a tarball
    and submit a explain task to PAI

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
    # TODO(typhoonzero): Do **NOT** create tmp table when the select statement
    # is like: "SELECT fields,... FROM table"
    data_table = table_ops.create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table
    params["explainer"] = explainer

    # format resultTable name to "db.table" to let the codegen form a
    # submitting argument of format "odps://project/tables/table_name"
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
    params["load"] = model_name

    label_column = model_params.get("label_col")
    params["label_column"] = label_column
    create_explain_result_table(datasource, data_table, result_table,
                                model_type, estimator, label_column)

    setup_explain_entry(params, model_type)
    prepare_archive(cwd, estimator, oss_model_path, params)

    cmd = get_pai_explain_cmd(datasource, project, oss_model_path, model_name,
                              data_table, result_table, model_type,
                              model_params,
                              "file://" + os.path.join(cwd, JOB_ARCHIVE_FILE),
                              "file://" + os.path.join(cwd, PARAMS_FILE),
                              label_column, cwd)

    submit_pai_task(cmd, datasource)
    table_ops.drop_tables([data_table], datasource)
