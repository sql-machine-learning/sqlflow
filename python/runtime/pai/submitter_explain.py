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

import datetime
import os

import oss2
import runtime.temp_file as temp_file
from runtime import db
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.feature.derivation import get_ordered_field_descs
from runtime.model import EstimatorType
from runtime.model.model import Model
from runtime.pai import cluster_conf, pai_model, table_ops
from runtime.pai.get_pai_tf_cmd import (ENTRY_FILE, JOB_ARCHIVE_FILE,
                                        PARAMS_FILE, get_pai_tf_cmd)
from runtime.pai.prepare_archive import prepare_archive
from runtime.pai.submit_pai_task import submit_pai_task
from runtime.pai_local.try_run import try_pai_local_run
from runtime.step.create_result_table import create_explain_table
from runtime.step.tensorflow.explain import print_image_as_base64_html


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
                        job_file, params_file, label_name):
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
                                             label_name)
    else:
        conf = cluster_conf.get_cluster_config(model_params)
        cmd = get_pai_tf_cmd(conf, job_file, params_file, ENTRY_FILE,
                             model_name, oss_model_path, data_table, "",
                             result_table, project)
    return cmd


def add_env_to_params(params, env_name, param_name):
    env = os.getenv(env_name)
    assert env, "%s cannot be empty" % env
    params[param_name] = env


def print_oss_image(oss_dest, oss_ak, oss_sk, oss_endpoint, oss_bucket_name):
    auth = oss2.Auth(oss_ak, oss_sk)
    bucket = oss2.Bucket(auth, oss_endpoint, oss_bucket_name)

    with temp_file.TemporaryDirectory(as_cwd=True):
        local_file_name = "summary.png"
        bucket.get_object_to_file(oss_dest, local_file_name)
        print_image_as_base64_html(local_file_name)


def submit_pai_explain(datasource,
                       original_sql,
                       select,
                       model,
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
        model: string
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

    # format resultTable name to "db.table" to let the codegen form a
    # submitting argument of format "odps://project/tables/table_name"
    project = table_ops.get_project(datasource)
    if result_table:
        if result_table.count(".") == 0:
            result_table = "%s.%s" % (project, result_table)
        params["result_table"] = result_table

    # used to save the explain image
    timestamp = datetime.datetime.now().strftime("%Y%m%d%H%M%S")
    params["oss_dest"] = "explain_images/%s/%s" % (user, timestamp)
    add_env_to_params(params, "SQLFLOW_OSS_AK", "oss_ak")
    add_env_to_params(params, "SQLFLOW_OSS_SK", "oss_sk")
    add_env_to_params(params, "SQLFLOW_OSS_ALISA_ENDPOINT", "oss_endpoint")
    add_env_to_params(params, "SQLFLOW_OSS_ALISA_BUCKET", "oss_bucket_name")

    meta = Model.load_metadata_from_db(datasource, model)
    model_type = meta.get_type()
    estimator = meta.get_meta("class_name")
    label_name = model_params.get("label_col")
    if label_name is None:
        label_column = meta.get_meta("label")
        if label_column is not None:
            label_name = label_column.get_field_desc()[0].name

    setup_explain_entry(params, model_type)

    oss_model_path = pai_model.get_oss_model_save_path(datasource,
                                                       model,
                                                       user=user)

    # TODO(typhoonzero): Do **NOT** create tmp table when the select statement
    # is like: "SELECT fields,... FROM table"
    with table_ops.create_tmp_tables_guard(select, datasource) as data_table:
        params["pai_table"] = data_table

        # Create explain result table
        if result_table:
            conn = db.connect_with_data_source(datasource)
            feature_columns = meta.get_meta("features")
            estimator_string = meta.get_meta("class_name")
            field_descs = get_ordered_field_descs(feature_columns)
            feature_column_names = [fd.name for fd in field_descs]
            create_explain_table(conn, meta.get_type(), explainer,
                                 estimator_string, result_table,
                                 feature_column_names)
            conn.close()

        if not try_pai_local_run(params, oss_model_path):
            with temp_file.TemporaryDirectory(prefix="sqlflow",
                                              dir="/tmp") as cwd:
                prepare_archive(cwd, estimator, oss_model_path, params)
                cmd = get_pai_explain_cmd(
                    datasource, project, oss_model_path, model, data_table,
                    result_table, model_type, model_params,
                    "file://" + os.path.join(cwd, JOB_ARCHIVE_FILE),
                    "file://" + os.path.join(cwd, PARAMS_FILE), label_name)
                submit_pai_task(cmd, datasource)

    print_oss_image(params["oss_dest"], params["oss_ak"], params["oss_sk"],
                    params["oss_endpoint"], params["oss_bucket_name"])
