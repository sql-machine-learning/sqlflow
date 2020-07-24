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
import shutil
import string
import subprocess
import tempfile
from os import path

from runtime import db, oss
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai import cluster_conf
from runtime.pai.kmeans import get_train_kmeans_pai_cmd
from runtime.pai.random_forest import get_train_random_forest_pai_cmd

LIFECYCLE_ON_TMP_TABLE = 7
JOB_ARCHIVE_FILE = "job.tar.gz"
PARAMS_FILE = "params.txt"
TRAIN_PARAMS_FILE = "train_params.pkl"
ENTRY_FILE = "entry.py"

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

XGB_REQUIREMENT = TF_REQUIREMENT + """
xgboost==0.82
sklearn2pmml==0.56.0
"""


def get_requirement(model_name):
    """Get required python package according to estimator name

    Args:
        estimator: name of the model

    Returns:
        A string with multilines, each line is a requirement
    """
    if model_name.lower().startswith("xgboost"):
        return XGB_REQUIREMENT
    else:
        return TF_REQUIREMENT


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
    if not select:
        return None
    conn = db.connect_with_data_source(datasource)
    project = get_project(datasource)
    tmp_tb_name = gen_rand_string()
    create_sql = "CREATE TABLE %s LIFECYCLE %s AS %s" % (
        tmp_tb_name, LIFECYCLE_ON_TMP_TABLE, select)
    # (NOTE: lhw) maxcompute conn doesn't support close
    # we should unify db interface
    if not db.execute(conn, create_sql):
        raise SQLFlowDiagnostic("Can't crate tmp table for %s" % select)
    return "%s.%s" % (project, tmp_tb_name)


def drop_tmp_tables(tables, datasource):
    """Drop given tables in datasource"""
    conn = db.connect_with_data_source(datasource)
    try:
        for table in tables:
            if table != "":
                drop_sql = "DROP TABLE %s" % table
                db.execute(conn, drop_sql)
    except:
        # odps will clear table itself, so even fail here, we do
        # not need to raise error
        print("Encounter error on drop tmp table")


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
    return "oss://%s/%s" % (oss.SQLFLOW_MODELS_BUCKET, model_full_path)


def drop_pai_model(datasource, model_name):
    """Drop PAI model

    Args:
        datasource: current datasource
        model_name: name of the model to drop
    """
    dsn = get_datasource_dsn(datasource)
    user, passwd, address, database = db.parseMaxComputeDSN(dsn)
    cmd = "drop offlinemodel if exists %s" % model_name
    subprocess.run([
        "odpscmd", "-u", user, "-p", passwd, "--project", database,
        "--endpoint", address, "-e", cmd
    ],
                   check=True)


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
        file.write("sqlflow_oss_modeldir=\"%s\"\n" % oss_model_url)
        file.flush()


def find_python_module_path(module):
    """Find the location of a given python package

    Args:
        module: given Python module

    Returns:
        The path of the Python module
    """
    proc = os.popen("python -c \"import %s;print(%s.__path__[0])\"" %
                    (module, module))
    output = proc.readline()
    return output.strip()


def copy_python_package(module, dest):
    """Copy given Python module to dist

    Args:
        module: The module to copy
        dest: the destination directory
    """
    module_path = find_python_module_path(module)
    if not module_path:
        raise SQLFlowDiagnostic("Can't find module %s" % module)
    shutil.copytree(module_path, path.join(dest, path.basename(module_path)))


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
    dsn = get_datasource_dsn(datasource)
    user, passwd, address, project = db.parseMaxComputeDSN(dsn)
    cmd = [
        "odpscmd", "--instance-priority", "9", "-u", user, "-p", passwd,
        "--project", project, "--endpoint", address, "-e", pai_cmd
    ]
    print(" ".join(cmd))
    if subprocess.call(cmd) != 0:
        raise SQLFlowDiagnostic("Execute odps cmd fail: cmd is %s" %
                                " ".join(cmd))


def get_oss_model_save_path(datasource, model_name):
    if not model_name:
        return None
    dsn = get_datasource_dsn(datasource)
    user, _, _, project = db.parseMaxComputeDSN(dsn)
    user = user or "unknown"
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
    # empty list param will raise error
    if len(object_path_list) > 0:
        bucket.batch_delete_objects(object_path_list)


def clean_oss_model_path(oss_path):
    bucket = oss.get_models_bucket()
    delete_oss_dir_recursive(bucket, oss_path)


def max_compute_table_url(table):
    parts = table.split(".")
    if len(parts) != 2:
        raise SQLFlowDiagnostic("odps table: %s should be format db.table" %
                                table)
    return "odps://%s/tables/%s" % (parts[0], parts[1])


def get_pai_tf_cmd(cluster_config, tarball, params_file, entry_file,
                   model_name, oss_model_path, train_table, val_table,
                   res_table, project):
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

    Retruns:
        The cmd to run on PAI
    """
    job_name = "_".join(["sqlflow", model_name]).replace(".", "_")
    cf_quote = json.dumps(cluster_config).replace("\"", "\\\"")

    # submit table should format as: odps://<project>/tables/<table >,odps://<project>/tables/<table > ...
    submit_tables = max_compute_table_url(train_table)
    if train_table != val_table and val_table:
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
           "-Dscript=%s -DentryFile=%s -Dtables=%s %s -DhyperParameters='%s'"
           ) % (job_name, tarball, entry_file, submit_tables, output_tables,
                params_file)

    # format the oss checkpoint path with ARN authorization.
    oss_checkpoint_configs = os.getenv("SQLFLOW_OSS_CHECKPOINT_CONFIG")
    if not oss_checkpoint_configs:
        raise SQLFlowDiagnostic(
            "need to configure SQLFLOW_OSS_CHECKPOINT_CONFIG when submitting to PAI"
        )
    ckpt_conf = json.loads(oss_checkpoint_configs)
    model_url = get_oss_model_url(oss_model_path)
    role_name = get_project_role_name(project)
    # format the oss checkpoint path with ARN authorization.
    oss_checkpoint_path = "%s/?role_arn=%s/%s&host=%s" % (
        model_url, ckpt_conf["arn"], role_name, ckpt_conf["host"])
    cmd = "%s -DcheckpointDir='%s'" % (cmd, oss_checkpoint_path)

    if cluster_config["worker"]["count"] > 1:
        cmd = "%s -Dcluster=\"%s\"" % (cmd, cf_quote)
    else:
        cmd = "%s -DgpuRequired='%d'" % (cmd, cluster_config["worker"]["gpu"])
    return cmd


def get_project_role_name(project):
    """Get oss role form project name.
    A valid role name contains letters and numbers only.
    The prefix 'pai2oss' of the role name denotes PAI access OS

    Args:
        project: string
            project name

    Returns:
        role name for the project
    """
    return "pai2oss" + "".join(x for x in project.lower()
                               if x in string.ascii_lowercase + string.digits)


def prepare_archive(cwd, conf, project, estimator, model_name, train_tbl,
                    val_tbl, model_save_path, train_params):
    """package needed resource into a tarball"""
    create_pai_hyper_param_file(cwd, PARAMS_FILE, model_save_path)

    with open(path.join(cwd, TRAIN_PARAMS_FILE), "wb") as param_file:
        pickle.dump(train_params, param_file, protocol=2)

    with open(path.join(cwd, "requirements.txt"), "w") as require:
        require.write(get_requirement(estimator))

    # copy entry.py to top level directory, so the package name `xgboost`
    # and `tensorflow` in runtime.pai will not conflict with the global ones
    shutil.copyfile(path.join(path.dirname(__file__), ENTRY_FILE),
                    path.join(cwd, ENTRY_FILE))
    copy_python_package("runtime", cwd)
    copy_python_package("sqlflow_models", cwd)
    copy_custom_package(estimator, cwd)

    args = [
        "tar", "czf", JOB_ARCHIVE_FILE, ENTRY_FILE, "runtime",
        "sqlflow_models", "requirements.txt", TRAIN_PARAMS_FILE
    ]
    if subprocess.call(args, cwd=cwd) != 0:
        raise SQLFlowDiagnostic("Can't zip resource")


def save_model_to_sqlfs(datasource, model_oss_path, model_name):
    # (TODO: save model to sqlfs)
    pass


# (TODO: lhw) adapt this interface after we do feature derivation in Python
def submit_pai_train(datasource, estimator_string, select, validation_select,
                     model_params, model_name, pre_trained_model,
                     **train_params):
    """This function submit PAI-TF train task to PAI platform

    Args:
        datasource: string
            Like: odps://access_id:access_key@service.com/api?curr_project=test_ci&scheme=http
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
    prepare_archive(cwd, conf, project, estimator_string, model_name,
                    train_table, val_table, path_to_save, params)

    # submit pai task to execute the training
    if estimator_string.lower().startswith("randomforests"):
        cmd = get_train_random_forest_pai_cmd(
            model_name, train_table, model_params,
            train_params["feature_column_names"],
            train_params["label_meta"]["feature_name"])
    elif estimator_string.lower().startswith("kmeans"):
        cmd = get_train_kmeans_pai_cmd(datasource, model_name, train_table,
                                       model_params,
                                       train_params["feature_column_names"])
    else:
        cmd = get_pai_tf_cmd(conf,
                             "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
                             "file://" + path.join(cwd, PARAMS_FILE),
                             ENTRY_FILE, model_name, path_to_save, train_table,
                             val_table, "", project)
    submit_pai_task(cmd, datasource)

    # save trained model to sqlfs
    save_model_to_sqlfs(datasource, path_to_save, model_name)
    drop_tmp_tables([train_table, val_table], datasource)


def get_oss_saved_model_type_and_estimator(model_name, project):
    """Get oss model type and estimator name, model can be:
    1. PAI ML models: model is saved by pai
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
    bucket = oss.get_models_bucket()
    tf = bucket.object_exists(model_name + "/tensorflow_model_desc")
    if tf:
        modelType = oss.MODEL_TYPE_TF
        bucket.get_object_to_file(
            model_name + "/tensorflow_model_desc_estimator",
            "tmp_estimator_name")
        with open("tmp_estimator_name") as file:
            estimator = file.readline()
        return modelType, estimator

    xgb = bucket.object_exists(model_name + "/xgboost_model_desc")
    if xgb:
        modelType = oss.MODEL_TYPE_XGB
        return modelType, "xgboost"

    return oss.MODEL_TYPE_PAIML, ""


def get_pai_predict_cmd(cluster_conf, datasource, project, oss_model_path,
                        model_name, predict_table, result_table, model_type,
                        cwd):
    """Get predict command for PAI task

    Args:
        cluster_conf: PAI cluster configuration
        datasource: current datasource
        project: current project
        oss_model_path: the place to load model
        model_name: model used to do prediction
        predict_table: where to store the tmp prediction data set
        result_table: prediction result
        model_type: type of th model, see also get_oss_saved_model_type
        cwd: current working dir

    Returns:
        The command to submit PAI prediction task
    """
    # NOTE(typhoonzero): for PAI machine learning toolkit predicting, we can not load the TrainStmt
    # since the model saving is fully done by PAI. We directly use the columns in SELECT
    # statement for prediction, error will be reported by PAI job if the columns not match.
    conn = db.connect_with_data_source(datasource)
    if model_type == oss.MODEL_TYPE_PAIML:
        schema = db.get_table_schema(conn, predict_table)
        result_fields = [col[0] for col in schema]
        return ('''pai -name prediction -DmodelName="%s"  '''
                '''-DinputTableName="%s"  -DoutputTableName="%s"  '''
                '''-DfeatureColNames="%s"  -DappendColNames="%s"''') % (
                    model_name, predict_table, result_table,
                    ",".join(result_fields), ",".join(result_fields))
    else:
        schema = db.get_table_schema(conn, result_table)
        result_fields = [col[0] for col in schema]
        # For TensorFlow and XGBoost, we build a pai-tf cmd to submit the task
        return get_pai_tf_cmd(cluster_conf,
                              "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
                              "file://" + path.join(cwd, PARAMS_FILE),
                              ENTRY_FILE, model_name, oss_model_path,
                              predict_table, "", result_table, project)


def create_predict_result_table(datasource, select, result_table, label_column,
                                train_label_column, model_type):
    """Create predict result table with given name and label column

    Args:
        datasource: current datasource
        select: sql statement to get prediction data set
        result_table: the table name to save result
        label_column: name of the label column, if not exist in select
            result, we will add a int column in the result table
        train_label_column: name of the label column when training
        model_type: type of model defined in runtime.oss
    """
    conn = db.connect_with_data_source(datasource)
    db.execute(conn, "DROP TABLE IF EXISTS %s" % result_table)
    # PAI ml will create result table itself
    if model_type == oss.MODEL_TYPE_PAIML:
        return

    create_table_sql = "CREATE TABLE %s AS SELECT * FROM %s LIMIT 0" % (
        result_table, select)
    db.execute(conn, create_table_sql)

    # if label is not in data table, add a int column for it
    schema = db.get_table_schema(conn, result_table)
    col_type = "INT"
    for (name, ctype) in schema:
        if name == train_label_column or name == label_column:
            col_type = ctype
            break
    col_names = [col[0] for col in schema]
    if label_column not in col_names:
        db.execute(
            conn, "ALTER TABLE %s ADD %s %s" %
            (result_table, label_column, col_type))
    if train_label_column != label_column and train_label_column in col_names:
        db.execute(
            conn, "ALTER TABLE %s DROP COLUMN %s" %
            (result_table, train_label_column))


def setup_predict_entry(params, model_type):
    """Setup PAI prediction entry function according to model type"""
    if model_type == oss.MODEL_TYPE_TF:
        params["entry_type"] = "predict_tf"
    elif model_type == oss.MODEL_TYPE_PAIML:
        params["entry_type"] = "predict_paiml"
    elif model_type == oss.MODEL_TYPE_XGB:
        params["entry_type"] = "predict_xgb"
    else:
        raise SQLFlowDiagnostic("unsupported model type: %d" % model_type)


def submit_pai_predict(datasource, select, result_table, label_column,
                       model_name, model_attrs):
    """This function pack needed params and resource to a tarball
    and submit a prediction task to PAI

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
    # TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
    # "SELECT fields,... FROM table"
    data_table = create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    # format resultTable name to "db.table" to let the codegen form a submitting
    # argument of format "odps://project/tables/table_name"
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

    conf = cluster_conf.get_cluster_config(model_attrs)
    prepare_archive(cwd, conf, project, estimator, model_name, data_table, "",
                    oss_model_path, params)

    cmd = get_pai_predict_cmd(conf, datasource, project, oss_model_path,
                              model_name, data_table, result_table, model_type,
                              cwd)
    submit_pai_task(cmd, datasource)
    drop_tmp_tables([data_table], datasource)


def get_create_shap_result_sql(conn, data_table, result_table, label_column):
    """Get a sql statement which create a result table for SHAP

    Args:
        conn: a database connection
        data_table: table name to read data from
        result_table: result table name
        label_column: column name of label 

    Returns:
        a sql statement to create SHAP result table
    """
    schema = db.get_table_schema(conn, data_table)
    fields = ["%s STRING" % f[0] for f in schema if f[0] != label_column]
    return "CREATE TABLE IF NOT EXISTS %s (%s)" % (result_table,
                                                   ",".join(fields))


# (TODO: lhw) This function is a common tool for prediction
# on all platforms, we need to move it to a new file
def create_explain_result_table(datasource, data_table, result_table,
                                model_type, estimator, label_column):
    """Create explain result table from given datasource

    Args:
        datasource: current datasource
        data_table: input data table name
        result_table: table name to store the result
        model_type: type of the model to use
        estimator: estimator class if the model is TensorFlow estimator
        label_column: column name of the predict label
    """
    conn = db.connect_with_data_source(datasource)
    drop_stmt = "DROP TABLE IF EXISTS %s" % result_table
    db.execute(conn, drop_stmt)

    create_stmt = ""
    if model_type == oss.MODEL_TYPE_PAIML:
        return
    elif model_type == oss.MODEL_TYPE_TF:
        if estimator.startswith("BoostedTrees"):
            column_def = ""
            if conn.driver == "mysql":
                column_def = "(feature VARCHAR(255), dfc FLOAT, gain FLOAT)"
            else:
                # Hive & MaxCompute
                column_def = "(feature STRING, dfc STRING, gain STRING)"
            create_stmt = "CREATE TABLE IF NOT EXISTS %s %s;" % (result_table,
                                                                 column_def)
        else:
            if not label_column:
                raise SQLFlowDiagnostic(
                    "need to specify WITH label_col=lable_col_name "
                    "when explaining deep models")
            create_stmt = get_create_shap_result_sql(conn, data_table,
                                                     result_table,
                                                     label_column)
    elif model_type == oss.MODEL_TYPE_XGB:
        if not label_column:
            raise SQLFlowDiagnostic(
                "need to specify WITH label_col=lable_col_name "
                "when explaining xgboost models")
        create_stmt = get_create_shap_result_sql(conn, data_table,
                                                 result_table, label_column)
    else:
        raise SQLFlowDiagnostic(
            "not supported modelType %d for creating Explain result table" %
            model_type)

    if not db.execute(conn, create_stmt):
        raise SQLFlowDiagnostic("Can't create explain result table")


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
    # NOTE(typhoonzero): for PAI random forests predicting, we can not load the TrainStmt
    # since the model saving is fully done by PAI. We directly use the columns in SELECT
    # statement for prediction, error will be reported by PAI job if the columns not match.
    if not label_column:
        raise SQLFlowDiagnostic("must specify WITH label_column when using "
                                "pai random forest to explain models")

    conn = db.connect_with_data_source(datasource)
    # drop result table if exists
    db.execute(conn, "DROP TABLE IF EXISTS %s;" % result_table)
    schema = db.get_table_schema(conn, data_table)
    fields = [f[0] for f in schema if f[0] != label_column]
    return (
        '''pai -name feature_importance -project algo_public '''
        '''-DmodelName="%s" -DinputTableName="%s"  '''
        '''-DoutputTableName="%s" -DlabelColName="%s" -DfeatureColNames="%s" '''
    ) % (model_name, data_table, result_table, label_column, ",".join(fields))


def submit_pai_explain(datasource, select, result_table, model_name,
                       model_attrs):
    """This function pack need params and resource to a tarball
    and submit a explain task to PAI

    Args:
        datasource: current datasource
        select: sql statement to get explain data set
        result_table: the table name to save result
        model_name: model used to do prediction
        model_params: dict, Params for training, crossponding to WITH clause
    """
    params = dict(locals())

    cwd = tempfile.mkdtemp(prefix="sqlflow", dir="/tmp")
    # TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
    # "SELECT fields,... FROM table"
    data_table = create_tmp_table_from_select(select, datasource)
    params["data_table"] = data_table

    # format resultTable name to "db.table" to let the codegen form a submitting
    # argument of format "odps://project/tables/table_name"
    project = get_project(datasource)
    if result_table.count(".") == 0:
        result_table = "%s.%s" % (project, result_table)

    oss_model_path = get_oss_model_save_path(datasource, model_name)
    model_type, estimator = get_oss_saved_model_type_and_estimator(
        oss_model_path, project)
    params["oss_model_path"] = oss_model_path

    label_column = model_attrs.get("label_col")
    params["label_column"] = label_column
    create_explain_result_table(datasource, data_table, result_table,
                                model_type, estimator, label_column)

    conf = cluster_conf.get_cluster_config(model_attrs)

    if model_type == oss.MODEL_TYPE_PAIML:
        cmd = get_explain_random_forests_cmd(datasource, model_name,
                                             data_table, result_table,
                                             label_column)
    else:
        if model_type == oss.MODEL_TYPE_XGB:
            params["entry_type"] = "explain_xgb"
        else:
            params["entry_type"] = "explain_tf"
        prepare_archive(cwd, conf, project, estimator, model_name, data_table,
                        "", oss_model_path, params)
        cmd = get_pai_tf_cmd(conf,
                             "file://" + path.join(cwd, JOB_ARCHIVE_FILE),
                             "file://" + path.join(cwd, PARAMS_FILE),
                             ENTRY_FILE, model_name, oss_model_path,
                             data_table, "", result_table, project)
    submit_pai_task(cmd, datasource)
    drop_tmp_tables([data_table], datasource)
