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
from os import path

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.model import EstimatorType, oss
from runtime.pai import cluster_conf
# yapf: disable
from runtime.pai.submitter import (ENTRY_FILE, JOB_ARCHIVE_FILE, PARAMS_FILE,
                                   clean_oss_model_path,
                                   create_evaluate_result_table,
                                   create_explain_result_table,
                                   create_predict_result_table,
                                   create_tmp_table_from_select,
                                   create_train_and_eval_tmp_table,
                                   drop_tables, gen_rand_string,
                                   get_evaluate_metrics,
                                   get_oss_model_save_path,
                                   get_oss_saved_model_type_and_estimator,
                                   get_pai_explain_cmd, get_pai_predict_cmd,
                                   get_pai_tf_cmd, get_pai_train_cmd,
                                   get_project, prepare_archive,
                                   save_model_to_sqlfs, setup_explain_entry,
                                   setup_predict_entry)

# yapf: enable

AlisaTaskTypePAI = 0
# AlisaTaskTypePyODPS is PyODPS task in the Alisa task enumeration
AlisaTaskTypePyODPS = 1


def getAlisaBucket():
    """Get Alisa oss bucket, this function get params from env variables"""
    ep = os.getenv("SQLFLOW_OSS_ALISA_ENDPOINT")
    ak = os.getenv("SQLFLOW_OSS_AK")
    sk = os.getenv("SQLFLOW_OSS_SK")
    bucketName = os.getenv("SQLFLOW_OSS_ALISA_BUCKET")

    if ep == "" or ak == "" or sk == "":
        return SQLFlowDiagnostic(
            "should define SQLFLOW_OSS_ALISA_ENDPOINT, "
            "SQLFLOW_OSS_ALISA_BUCKET, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK "
            "when using submitter alisa")

    return oss.get_bucket(bucketName, ak, sk, endpoint=ep)


def upload_resource(file_path, oss_obj_name, bucket):
    """Upload resource from file_path to oss with given oss_obj_name

    Args:
        file_path: file path to upload
        oss_obj_name: name of uploaded oss object
        bucket: oss bucket to store the object

    Returns:
        The oss object uri to access the uploaded resource
    """

    resource_oss_url = "https://%s.%s/%s" % (bucket.bucket_name,
                                             bucket.endpoint, oss_obj_name)
    bucket.put_object_from_file(oss_obj_name, file_path)
    return resource_oss_url


# (TODO: lhw) This is a placeholder, we should use alisa db api
def parse_alisa_config(datasource):
    return {
        "POPAccessID": "",
        "POPAccessSecret": "",
        "POPURL": "",
        "POPScheme": "",
        "Env": {},
        "With": {},
        "Verbose": False,
        "Project": ""
    }


# (TODO: lhw) This is a placeholder, we should use alisa db api
def alisa_execute(submit_code, cfg):  # noqa W0613 C0116
    pass


def submit_alisa_task(datasource, task_type, submit_code, args):
    """Submit an Alias task

    Args:
        datasource: the datasource to use
        task_type: AlisaTaskTypePAI or AlisaTaskTypePyODPS
        submit_code: the code to submit a PAI task
        args: map of arguments, like codeResourceURL and others
    """
    cfg = parse_alisa_config(datasource)

    if task_type == AlisaTaskTypePAI:
        cfg["Env"]["RES_DOWNLOAD_URL"] = (
            """[{"downloadUrl":"%s", "resourceName":"%s"}, """
            """{"downloadUrl":"%s", "resourceName":"%s"}]""") % (
                args["codeResourceURL"], args["resourceName"],
                args["paramsResourceURL"], args["paramsFile"])

    cfg["Verbose"] = True

    if task_type == AlisaTaskTypePAI:
        alisa_execute(submit_code, None)
    elif task_type == AlisaTaskTypePyODPS:
        alisa_execute(submit_code, args)
    else:
        raise SQLFlowDiagnostic("Unknown AlisaTaskType %d" % task_type)


def upload_resource_and_submit_alisa_task(datasource, job_file, params_file,
                                          pai_cmd):
    """Upload the packed resource and submit an Alisa task

    Args:
        datasource: the datasource to use
        job_file: zipped local resource, including code and libs
        params_file: the extra params file
        pai_cmd: the command to run on PAI
    """
    oss_code_obj = gen_rand_string(16)
    bucket = getAlisaBucket()
    oss_code_url = upload_resource(job_file, oss_code_obj, bucket)

    # upload params.txt for additional training parameters.
    oss_params_obj = gen_rand_string(16)
    oss_params_url = upload_resource(params_file, oss_params_obj, bucket)
    conf = {
        "codeResourceURL": oss_code_url,
        "paramsResourceURL": oss_params_url,
        "resourceName": job_file,
        "paramsFile": params_file,
    }
    submit_alisa_task(datasource, AlisaTaskTypePAI, pai_cmd, conf)

    bucket.delete_object(oss_code_obj)
    bucket.delete_object(oss_params_obj)


# (TODO: lhw) adapt this interface after we do feature derivation in Python
def submit_alisa_train(datasource, estimator_string, select, validation_select,
                       model_params, model_name, pre_trained_model,
                       **train_params):
    """This function submit PAI-TF train task to PAI platform through Alisa

    Args:
        datasource: string
            Like: alisa://access_id:access_key@service.com/api?
                curr_project=test_ci&scheme=http
        estimator_string: string
            Tensorflow estimator name, Keras class name, or XGBoost
        select: string
            The SQL statement for selecting data for train
        validation_select: string
            Ths SQL statement for selecting data for validation
        model_params: dict
            Params for training, crossponding to WITH clause
        pre_trained_model: string
            The pre-trained model name to load
        train_params: dict
            Extra train params, they will be passed to runtime.tensorflow.train
    """

    # prepare params for tensorflow train,
    # the params will be pickled into train_params.pkl
    params = dict(locals())
    del params["train_params"]
    params.update(train_params)

    if estimator_string.lower().startswith("xgboost"):
        params["entry_type"] = "train_xgb"
    else:
        params["entry_type"] = "train_tf"

    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")

    train_table, val_table = create_train_and_eval_tmp_table(
        select, validation_select, datasource)
    params["pai_table"], params["pai_val_table"] = train_table, val_table

    # clean target dir
    path_to_save = get_oss_model_save_path(datasource, model_name)
    path_to_load = get_oss_model_save_path(datasource, pre_trained_model)
    params["oss_model_dir"] = path_to_save

    if path_to_load == "" or path_to_load != path_to_save:
        clean_oss_model_path(path_to_save + "/")

    # zip all required resource to a tarball
    prepare_archive(cwd, estimator_string, path_to_save, params)

    # submit pai task to execute the training
    cmd = get_pai_train_cmd(datasource, estimator_string, model_name,
                            train_table, val_table, model_params, train_params,
                            path_to_save, "file://@@%s" % JOB_ARCHIVE_FILE,
                            "file://@@%s" % PARAMS_FILE, cwd)
    upload_resource_and_submit_alisa_task(
        datasource, "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
        "file://" + path.join(cwd, PARAMS_FILE), cmd)

    # save trained model to sqlfs
    save_model_to_sqlfs(datasource, path_to_save, model_name)
    drop_tables([train_table, val_table], datasource)


def submit_alisa_predict(datasource, select, result_table, label_column,
                         model_name, model_params):
    """This function pack needed params and resource to a tarball
    and submit a prediction task to PAI throught Alisa

    Args:
        datasource: current datasource
        select: sql statement to get prediction data set
        result_table: the table name to save result
        label_column: name of the label column, if not exist in select
        model_name: model used to do prediction
        model_params: dict, Params for training, crossponding to WITH clause
    """
    params = dict(locals())

    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")
    data_table = create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    # format resultTable name to "db.table" to let the
    # codegen form a submitting argument of format
    # "odps://project/tables/table_name"
    project = get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)

    oss_model_path = get_oss_model_save_path(datasource, model_name)
    params["oss_model_path"] = oss_model_path
    model_type, estimator = get_oss_saved_model_type_and_estimator(
        oss_model_path, project)
    setup_predict_entry(params, model_type)

    # (TODO:lhw) get train label column from model meta
    create_predict_result_table(datasource, data_table, result_table,
                                label_column, None, model_type)

    prepare_archive(cwd, estimator, oss_model_path, params)

    cmd = get_pai_predict_cmd(datasource, project, oss_model_path, model_name,
                              data_table, result_table, model_type,
                              model_params, "file://@@%s" % JOB_ARCHIVE_FILE,
                              "file://@@%s" % PARAMS_FILE, cwd)

    upload_resource_and_submit_alisa_task(
        datasource, "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
        "file://" + path.join(cwd, PARAMS_FILE), cmd)

    drop_tables([data_table], datasource)


def submit_alisa_explain(datasource, select, result_table, model_name,
                         model_params):
    """This function pack need params and resource to a tarball
    and submit a explain task to PAI through Alisa

    Args:
        datasource: current datasource
        select: sql statement to get explain data set
        result_table: the table name to save result
        model_name: model used to do prediction
        model_params: dict, Params for training, crossponding to WITH clause
    """
    params = dict(locals())

    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")
    # TODO(typhoonzero): Do **NOT** create tmp table when the select
    # statement is like: "SELECT fields,... FROM table"
    data_table = create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    # format resultTable name to "db.table" to let the codegen
    # form a submitting argument of format
    # "odps://project/tables/table_name"
    project = get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)

    oss_model_path = get_oss_model_save_path(datasource, model_name)
    model_type, estimator = get_oss_saved_model_type_and_estimator(
        oss_model_path, project)
    params["oss_model_path"] = oss_model_path

    label_column = model_params.get("label_col")
    params["label_column"] = label_column
    create_explain_result_table(datasource, data_table, result_table,
                                model_type, estimator, label_column)

    setup_explain_entry(params, model_type)
    prepare_archive(cwd, estimator, oss_model_path, params)

    cmd = get_pai_explain_cmd(datasource, project, oss_model_path, model_name,
                              data_table, result_table, model_type,
                              model_params, "file://@@%s" % JOB_ARCHIVE_FILE,
                              "file://@@%s" % PARAMS_FILE, label_column, cwd)
    upload_resource_and_submit_alisa_task(
        datasource, "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
        "file://" + path.join(cwd, PARAMS_FILE), cmd)
    drop_tables([data_table], datasource)


def submit_alisa_evaluate(datasource, model_name, select, result_table,
                          model_attrs):
    """Submit a PAI evaluation task through Alisa

    Args:
        datasource: current datasource
        model_name: model used to do evaluation
        select: sql statement to get evaluate data set
        result_table: the table name to save result
        model_params: dict, Params for training, crossponding to WITH claus
    """

    params = dict(locals())
    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")

    project = get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)
    oss_model_path = get_oss_model_save_path(datasource, model_name)
    params["oss_model_path"] = oss_model_path

    model_type, estimator = get_oss_saved_model_type_and_estimator(
        oss_model_path, project)
    if model_type == EstimatorType.PAIML:
        raise SQLFlowDiagnostic("PAI model evaluation is not supported yet.")

    data_table = create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    metrics = get_evaluate_metrics(model_type, model_attrs)
    params["metrics"] = metrics
    create_evaluate_result_table(datasource, result_table, metrics)

    conf = cluster_conf.get_cluster_config(model_attrs)

    if model_type == EstimatorType.XGBOOST:
        params["entry_type"] = "evaluate_xgb"
    else:
        params["entry_type"] = "evaluate_tf"
    prepare_archive(cwd, estimator, oss_model_path, params)
    cmd = get_pai_tf_cmd(conf, "file://@@%s" % JOB_ARCHIVE_FILE,
                         "file://@@%s" % PARAMS_FILE, ENTRY_FILE, model_name,
                         oss_model_path, data_table, "", result_table, project)
    upload_resource_and_submit_alisa_task(
        datasource, "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
        "file://" + path.join(cwd, PARAMS_FILE), cmd)
    drop_tables([data_table], datasource)
