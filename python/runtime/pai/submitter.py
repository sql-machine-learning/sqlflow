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

import json
import os
import pickle
import random
import string
from os import path

from .. import db
from ..tensorflow.diag import SQLFlowDiagnostic
from . import cluster_conf, model
from .entry import tensorflow as tensorflow_entry

LIFECYCLE_ON_TMP_TABLE = 7
JOB_ARCHIVE_FILE = "job.tar.gz"
PARAMS_FILE = "params.txt"
TRAIN_PARAMS_FILE = "train_params.pkl"
ENTRY_DIR = "sqlflow_submitter/pai/entry/"

TF_REQUIREMENT = """
adanet==0.8.0
numpy==1.16.2
pandas==0.24.2
plotille==3.7
seaborn==0.9.0
shap==0.28.5
scikit-learn==0.20.4
tensorflow-datasets==3.0.0
"""


def gen_rand_string(slen=16):
    """generate random string with given len

    Args:
        slen: int, the length of the output string

    Returns:
        A random string with slen length
    """
    return ''.join(random.sample(string.ascii_letters + string.digits, slen))


def create_tmp_table_from_select(select, datasource):
    """Create temp table for given select query

    Args:
        select: string, the selection statement
        datasource: string, the datasource to connect
    """
    if len(select.strip()) == 0:
        return ""
    conn = db.connect_with_data_source(datasource)
    project = get_project(datasource)
    tmp_tb_name = gen_rand_string()
    create_sql = "CREATE TABLE %s LIFECYCLE %s AS %s" % (
        tmp_tb_name, LIFECYCLE_ON_TMP_TABLE, select)
    cursor = conn.cursor()
    cursor.execute(create_sql)
    conn.commit()
    cursor.close()
    conn.close()
    return "%s.%s" % (project, tmp_tb_name)


def drop_tmp_tables(tables, datasource):
    """Drop given tables in datasource"""
    conn = db.connect_with_data_source(datasource)
    cursor = conn.cursor()
    for table in tables:
        if table != "":
            drop_sql = "DROP TABLE %s" % table
            cursor.execute(drop_sql)
    conn.commit()
    cursor.close()
    conn.close()


def create_train_and_eval_tmp_table(train_select, valid_select, datasource):
    train_table = create_tmp_table_from_select(train_select, datasource)
    valid_table = create_tmp_table_from_select(valid_select, datasource)
    return train_table, valid_table


def get_oss_model_url(model_full_path):
    """Get OSS model save url

    Args:
        model_full_path: string, the path in OSS bucket

    Returns:
        The OSS url of the model
    """
    return "oss://%s/%s" % (model.SQLFLOW_MODELS_BUCKET, model_full_path)


def create_pai_hyper_param_file(cwd, filename, model_path):
    """Create param needed by PAI training

    Args:
        cwd: current working dir
        filename: the output file name
        model_path: the model saving path
    """
    with open(path.join(cwd, filename), "w") as file:
        oss_ak = os.getenv("SQLFLOW_OSS_AK")
        oss_sk = os.getenv("SQLFLOW_OSS_SK")
        oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
        if oss_ak == "" or oss_sk == "" or oss_ep == "":
            raise SQLFlowDiagnostic(
                "must define SQLFLOW_OSS_AK, SQLFLOW_OSS_SK, "
                "SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")
        file.write("sqlflow_oss_ak=\"%s\"\n" % oss_ak)
        file.write("sqlflow_oss_sk=\"%s\"\n" % oss_sk)
        file.write("sqlflow_oss_ep=\"%s\"\n" % oss_ep)
        oss_model_url = get_oss_model_url(model_path)
        file.write("sqlflow_oss_modeldir=\"%s\"\n", oss_model_url)
        file.flush()


def find_python_module_path(module):
    """Find the location of a given python package

    Args:
        module: given Python module

    Returns:
        The path of the Python module
    """
    proc = os.popen("python -c import %s;print(%s.__path__[0])" %
                    (module, module))
    output = proc.readline()
    return output.strip()


def copy_python_package(module, dest):
    """Copy given Python module to dist

    Args:
        module: The module to copy
        dest: the destination directory
    """
    path = find_python_module_path(module)
    os.execl("cp", "-r", path, dest)


def copy_custom_package(estimator, dst):
    """Copy custom Python package to dest"""
    model_name_parts = estimator.split(".")
    pkg_name = model_name_parts[0]
    if (len(model_name_parts) == 2 and pkg_name != "sqlflow_models"
            and pkg_name != "xgboost"):
        copy_python_package(pkg_name, dst)


def submit_pai_task(pai_cmd, datasource):
    """Submit given cmd to PAI which manipulate datasource

    Args:
        pai_cmd: The command to submit
        datasource: The datasource this cmd will manipulate
    """
    user, passwd, address, project = db.parseMaxComputeDSN(datasource)
    os.execl("odpscmd", "--instance-priority", "9", "-u", user, "-p", passwd,
             "--project", project, "--endpoint", address, "-e", pai_cmd)


def get_oss_model_save_path(datasource, model_name):
    dsn = get_datasource_dsn(datasource)
    user, _, _, project = db.parseMaxComputeDSN(dsn)
    return "/".join([project, user, model_name])


def get_datasource_dsn(datasource):
    return datasource.split("://")[1]


def get_project(datasource):
    """Get the project info from given datasource

    Args:
        datasource: The odps url to extract project
    """
    dsn = get_datasource_dsn(datasource)
    _, _, _, project = db.parseMaxComputeDSN(dsn)
    return project


def delete_oss_dir_recursive(bucket, directory):
    """Recursively delete a directory on the OSS

    Args:
        bucket: bucket on OSS
        directory: the directory to delete
    """
    if not directory.endswith("/"):
        raise SQLFlowDiagnostic("dir to delete must end with /")

    loc = bucket.list_objects(prefix=directory, delimiter="/")
    object_path_list = []
    for obj in loc.object_list:
        object_path_list.append(obj.key)

    # delete sub dir first
    if len(loc.prefix_list) > 0:
        for sub_prefix in loc.prefix_list:
            delete_oss_dir_recursive(bucket, sub_prefix)
    bucket.batch_delete_objects(object_path_list)


def clean_oss_model_path(oss_path):
    bucket = model.get_models_bucket()
    delete_oss_dir_recursive(bucket, oss_path)


def max_compute_table_url(table):
    parts = table.split(".")
    if len(parts) != 2:
        raise SQLFlowDiagnostic("odps table: %s should be format db.table" %
                                table)
    return "odps://%s/tables/%s" % (parts[0], parts[1])


def get_pai_tf_cmd(cluster_config, tarball, params_file, entry_file,
                   model_name, oss_model_path, train_table, val_table,
                   res_table, project, cwd):
    """Get PAI-TF cmd for training

    Args:
        cluster_config: PAI cluster config
        tarball: the zipped resource name
        params_file: PAI param file name
        entry_file: entry file in the tarball
        model_name: trained model name
        oss_model_path: path to save the model
        train_table: train data table
        val_table: evaluate data table
        res_table: table to save train model, if given
        project: current odps project
        cwd: current working dir

    Retruns:
        The cmd to run on PAI
    """
    job_name = "_".join(["sqlflow", model_name]).replace(".", "_")
    cf_quote = json.dumps(cluster_config).replace("\"", "\\\"")

    # submit table should format as: odps://<project>/tables/<table >,odps://<project>/tables/<table > ...
    submit_tables = max_compute_table_url(train_table)
    if train_table != val_table and val_table != "":
        val_table = max_compute_table_url(val_table)
        submit_tables = "%s,%s" % (submit_tables, val_table)
    output_tables = ""
    if res_table != "":
        table = max_compute_table_url(res_table)
        output_tables = "-Doutputs=%s" % table

    # NOTE(typhoonzero): use - DhyperParameters to define flags passing OSS credentials.
    # TODO(typhoonzero): need to find a more secure way to pass credentials.
    cmd = ("pai -name tensorflow1150 -project algo_public_dev "
           "-DmaxHungTimeBeforeGCInSeconds=0 -DjobName=%s -Dtags=dnn "
           "-Dscript=%s -DentryFile=%s -Dtables=%s %s -DhyperParameters=\"%s\""
           ) % (job_name, tarball, entry_file, submit_tables, output_tables,
                params_file)

    # format the oss checkpoint path with ARN authorization.
    oss_checkpoint_configs = os.getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
    if oss_checkpoint_configs == "":
        raise SQLFlowDiagnostic(
            "need to configure SQLFLOW_OSS_CHECKPOINT_DIR when submitting to PAI"
        )
    ckpt_conf = json.loads(oss_checkpoint_configs)
    model_url = get_oss_model_url(oss_model_path)
    role_name = "pai2oss_%s" % project
    # format the oss checkpoint path with ARN authorization.
    oss_checkpoint_path = "%s/?role_arn=%s/%s&host=%s" % (
        model_url, ckpt_conf["Arn"], role_name, ckpt_conf["Host"])
    cmd = "%s -DcheckpointDir='%s'" % (cmd, oss_checkpoint_path)

    if cluster_config["worker"]["count"] > 1:
        cmd = "%s -Dcluster=\"%s\"" % (cmd, cf_quote)
    else:
        cmd = "%s -DgpuRequired='%d'" % (cmd, cluster_config["worker"]["gpu"])
    return cmd


def prepare_archive(cwd, conf, project, estimator, model_name, train_tbl,
                    val_tbl, model_save_path, train_params):
    """package needed resource into a tarball"""
    create_pai_hyper_param_file(cwd, PARAMS_FILE, model_save_path)

    with open(path.join(cwd, TRAIN_PARAMS_FILE), "w") as param_file:
        pickle.dump(train_params, param_file)

    with open(path.join(cwd, "requirements.txt"), "w") as require:
        require.write(TF_REQUIREMENT)
    copy_python_package("sqlflow_submitter", cwd)
    copy_python_package("sqlflow_models", cwd)
    copy_custom_package(estimator, cwd)

    os.execl("tar", "czf", JOB_ARCHIVE_FILE, "./sqlflow_submitter",
             "./sqlflow_models", "requirements.txt", TRAIN_PARAMS_FILE)


def save_model_to_sqlfs(datasource, model_oss_path, model_name):
    # (TODO: save model to sqlfs)
    pass


# (TODO: lhw) adapt this interface after we do feature derivation in Python
def submit_pytf_train(datasource, estimator, select, validation_select,
                      model_params, model_name, pre_trained_model,
                      **train_params):
    """This function submit PY-TF train task to PAI platform

    Args:
        datasource: string
            Like: odps://access_id:access_key@service.com/api?curr_project=test_ci&scheme=http
        estimator: string
            The name of tensorflow estimator
        select: string
            The SQL statement for selecting data for train
        validation_select: string
            Ths SQL statement for selecting data for validation
        model_params: dict
            Params for training, crossponding to WITH clause
        pre_trained_model: string
            The pre-trained model name to load
        train_params: dict
            Extra train params, they will be passed to sqlflow_submitter.tensorflow.train
    """

    # prepare params for tensorflow train,
    # the params will be pickled into train_params.pkl
    params = locals()
    del params["train_params"]
    params.update(train_params)
    params["entry_type"] = "train"

    cwd = os.getcwd()
    conf = cluster_conf.get_cluster_config(model_params)

    train_table, val_table = create_train_and_eval_tmp_table(
        select, validation_select, datasource)
    params["pai_table"], params["pai_val_table"] = train_table, val_table

    # clean target dir
    path_to_save = get_oss_model_save_path(datasource, model_name)
    path_to_load = get_oss_model_save_path(datasource, pre_trained_model)
    project = get_project(datasource)
    params["oss_model_dir"] = path_to_save

    if path_to_load == "" or path_to_load != path_to_save:
        clean_oss_model_path(path_to_save + "/")

    # zip all required resource to a tarball
    prepare_archive(cwd, conf, project, estimator, model_name, train_table,
                    val_table, path_to_save, params)

    # submit pai task to execute the training
    cmd = get_pai_tf_cmd(conf, JOB_ARCHIVE_FILE, PARAMS_FILE,
                         ENTRY_DIR + "tensorflow.py", model_name, path_to_save,
                         train_table, val_table, "", project, cwd)
    submit_pai_task(cmd, datasource)

    # save trained model to sqlfs
    save_model_to_sqlfs(datasource, path_to_save, model_name)
    drop_tmp_tables([train_table, val_table], datasource)


def get_oss_saved_model_type(model_name, project):
    """Get oss model type and estimator name, model can be:
    1. randomforests: model is saved by pai
    2. xgboost: on OSS with model file xgboost_model_desc
    3. PAI tensorflow models: on OSS with meta file: tensorflow_model_desc

    Args:
        model_name: the model to get info
        project: current odps project

    Returns:
        If model is TensorFlow model, return type and estimator name
        If model is XGBoost, or other PAI model, just return model type
    """
    # FIXME(typhoonzero): if the model not exist on OSS, assume it's a random forest model
    # should use a general method to fetch the model and see the model type.
    bucket = model.get_models_bucket()
    tf = bucket.object_exists(model_name + "/tensorflow_model_desc")
    if tf:
        modelType = model.MODEL_TYPE_TF
        bucket.get_object_to_file(
            model_name + "/tensorflow_model_desc_estimator",
            "tmp_estimator_name")
        with open("tmp_estimator_name") as file:
            estimator = file.readline()
        return modelType, estimator

    xgb = bucket.object_exists(model_name + "/xgboost_model_desc")
    if xgb:
        modelType = model.MODEL_TYPE_XGB
        return modelType, ""

    return model.MODEL_TYPE_PAIML, ""


def get_pai_predict_cmd(datasource, select, model_name, predict_table,
                        result_table, model_type):
    """Get predict command for PAI task

    Args:
        datasource: current datasource
        select: sql statement to get predict data set
        model_name: model used to do prediction
        predict_table: where to store the tmp prediction data set
        result_table: prediction result
        model_type: type of th model, see also get_oss_saved_model_type

    Returns:
        The command to submit PAI prediction task
    """
    # NOTE(typhoonzero): for PAI machine learning toolkit predicting, we can not load the TrainStmt
    # since the model saving is fully done by PAI. We directly use the columns in SELECT
    # statement for prediction, error will be reported by PAI job if the columns not match.
    schema = db.get_table_schema(datasource, result_table)
    result_fields = [col[0] for col in schema]

    if model_type == model.MODEL_TYPE_PAIML:
        return ('''pai -name prediction -DmodelName="%s"  '''
                '''-DinputTableName="%s"  DoutputTableName="%s"  '''
                '''-DfeatureColNames="%s"  -DappendColNames="%s"''') % (
                    model_name, predict_table, result_table,
                    ",".join(result_fields), ",".join(result_fields))
    else:
        # (TODO:lhw) add cmd for other model_type
        raise SQLFlowDiagnostic("not implemented")


def create_predict_result_table(datasource, select, result_table,
                                label_column):
    """Create predict result table with given name and label column

    Args:
        datasource: current datasource
        select: sql statement to get prediction data set
        result_table: the table name to save result
        label_column: name of the label column, if not exist in select
            result, we will add a int column in the result table
    """
    conn = db.connect_with_data_source(datasource)
    db.exec(conn, "DROP TABLE IF EXISTS %s" % result_table)
    create_table_sql = "CREATE TABLE %s AS SELECT * FROM %s LIMIT 0" % (
        result_table, select)
    if conn.driver == "hive":
        create_table_sql += (" ROW FORMAT DELIMITED FIELDS "
                             "TERMINATED BY \"\\001\" STORED AS TEXTFILE;")
    db.exec(conn, create_table_sql)

    # if label is not in data table, add a int column for it
    schema = db.get_table_schema(datasource, result_table)
    if not any(col[0] == label_column for col in schema):
        db.exec(conn,
                "ALTER TABLE %s ADD %sINT" % (result_table, label_column))


def submit_pytf_predict(datasource, select, result_table, label_column,
                        model_name, model_attrs):
    """This function pack need params and resource to a tarball
    and submit a prediction task to PAI

    Args:
        datasource: current datasource
        select: sql statement to get prediction data set
        result_table: the table name to save result
        label_column: name of the label column, if not exist in select
        model_name: model used to do prediction
        model_params: dict, Params for training, crossponding to WITH clause
    """
    params = locals()
    params["entry_type"] = "predict"

    cwd = os.getcwd()
    # TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
    # "SELECT fields,... FROM table"
    data_table = create_tmp_table_from_select(select, datasource)

    # format resultTable name to "db.table" to let the codegen form a submitting
    # argument of format "odps://project/tables/table_name"
    project = get_project(datasource)
    if result_table.count(".") == 1:
        result_table = "%s.%s" % (project, result_table)

    create_predict_result_table(datasource, data_table, result_table,
                                label_column)

    oss_model_path = get_oss_model_save_path(datasource, model_name)
    model_type, estimator = get_oss_saved_model_type(oss_model_path, project)

    conf = cluster_conf.get_cluster_config(model_attrs)
    prepare_archive(cwd, conf, project, estimator, model_name, data_table, "",
                    oss_model_path, params)

    cmd = get_pai_predict_cmd(datasource, select, model_name, data_table,
                              result_table, model_type)
    submit_pai_task(cmd, datasource)
