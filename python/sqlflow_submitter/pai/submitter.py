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
import random
import string
from os import path

from .. import db
from . import model

LIFECYCLE_ON_TMP_TABLE = 7
RESOURCE_NAME = "job.tar.gz"
ENTRY_FILE = "entry.py"


def gen_rand_string(slen=16):
    """generate random string with given len"""
    return ''.join(random.sample(string.ascii_letters + string.digits, slen))


def create_tmp_table_from_select(select, datasource):
    """create temp table for given select query"""
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
    return "oss://%s/%s" % (model.SQLFLOW_MODELS_BUCKET, model_full_path)


def create_pai_hyper_param_file(cwd, filename, model_path):
    with open(path.join(cwd, filename), "w") as file:
        oss_ak = os.getenv("SQLFLOW_OSS_AK")
        oss_sk = os.getenv("SQLFLOW_OSS_SK")
        oss_ep = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT")
        if oss_ak == "" or oss_sk == "" or oss_ep == "":
            print("must define SQLFLOW_OSS_AK, SQLFLOW_OSS_SK, "
                  "SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")
        file.write("sqlflow_oss_ak=\"{}\"\n" % oss_ak)
        file.write("sqlflow_oss_sk=\"{}\"\n" % oss_sk)
        file.write("sqlflow_oss_ep=\"{}\"\n" % oss_ep)
        oss_model_url = get_oss_model_url(model_path)
        file.write("sqlflow_oss_modeldir=\"%s\"\n", oss_model_url)
        file.flush()


def find_python_module_path(module):
    proc = os.popen("python -c import %s;print(%s.__path__[0])" %
                    (module, module))
    output = proc.readline()
    return output.strip()


def copy_python_package(module, dest):
    path = find_python_module_path(module)
    os.execl("cp", "-r", path, dest)


def copy_custom_package(estimator, dst):
    model_name_parts = estimator.split(".")
    pkg_name = model_name_parts[0]
    if (len(model_name_parts) == 2 and pkg_name != "sqlflow_models"
            and pkg_name != "xgboost"):
        copy_python_package(pkg_name, dst)


def achieve_resource(cwd, entry_code, requirements, tarball, estimator):
    """package needed resource to a tarball"""
    with open(path.join(cwd, ENTRY_FILE), "w") as entry:
        entry.write(entry_code)
    with open(path.join(cwd, "requirements.txt"), "w") as require:
        require.write(requirements)
    copy_python_package("sqlflow_submitter", cwd)
    copy_python_package("sqlflow_models", cwd)
    copy_custom_package(estimator, cwd)

    os.execl("tar", "czf", tarball, "./sqlflow_submitter", "./sqlflow_models",
             ENTRY_FILE, "requirements.txt")


def submit_pai_task(cwd, code, pai_cmd, requirements, estimator, datasource):
    achieve_resource(cwd, code, requirements, RESOURCE_NAME, estimator)
    user, passwd, address, project = db.parseMaxComputeDSN(datasource)
    os.execl("odpscmd", "--instance-priority", "9", "-u", user, "-p", passwd,
             "--project", project, "--endpoint", address, "-e", pai_cmd)


def get_model_save_path(datasource, model_name):
    user, _, _, project = db.parseMaxComputeDSN(datasource)
    return "/".join([project, user, model_name])


def get_project(datasource):
    _, _, _, project = db.parseMaxComputeDSN(datasource)
    return project


def delete_oss_dir_recursive(bucket, directory):
    """deleteDirRecursive recursively delete a directory on the OSS"""
    if not directory.endswith("/"):
        raise "dir to delete must end with /"

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


# (TODO: lhw) add train entry point
