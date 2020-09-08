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
import string

from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai import pai_model

JOB_ARCHIVE_FILE = "job.tar.gz"
PARAMS_FILE = "params.txt"
ENTRY_FILE = "entry.py"


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

    # submit table should format as: odps://<project>/tables/<table >,
    # odps://<project>/tables/<table > ...
    submit_tables = _max_compute_table_url(train_table)
    if train_table != val_table and val_table:
        val_table = _max_compute_table_url(val_table)
        submit_tables = "%s,%s" % (submit_tables, val_table)
    output_tables = ""
    if res_table != "":
        table = _max_compute_table_url(res_table)
        output_tables = "-Doutputs=%s" % table

    # NOTE(typhoonzero): use - DhyperParameters to define flags passing
    # OSS credentials.
    # TODO(typhoonzero): need to find a more secure way to pass credentials.
    cmd = ("pai -name tensorflow1150 -project algo_public_dev "
           "-DmaxHungTimeBeforeGCInSeconds=0 -DjobName=%s -Dtags=dnn "
           "-Dscript=%s -DentryFile=%s -Dtables=%s %s -DhyperParameters='%s'"
           ) % (job_name, tarball, entry_file, submit_tables, output_tables,
                params_file)

    # format the oss checkpoint path with ARN authorization, should use eval
    # because we use '''json''' in the workflow yaml file.
    oss_checkpoint_configs = eval(os.getenv("SQLFLOW_OSS_CHECKPOINT_CONFIG"))
    if not oss_checkpoint_configs:
        raise SQLFlowDiagnostic(
            "need to configure SQLFLOW_OSS_CHECKPOINT_CONFIG when "
            "submitting to PAI")
    ckpt_conf = json.loads(oss_checkpoint_configs)
    model_url = pai_model.get_oss_model_url(oss_model_path)
    role_name = _get_project_role_name(project)
    # format the oss checkpoint path with ARN authorization.
    oss_checkpoint_path = "%s/?role_arn=%s/%s&host=%s" % (
        model_url, ckpt_conf["arn"], role_name, ckpt_conf["host"])
    cmd = "%s -DcheckpointDir='%s'" % (cmd, oss_checkpoint_path)

    if cluster_config["worker"]["count"] > 1:
        cmd = "%s -Dcluster=\"%s\"" % (cmd, cf_quote)
    else:
        cmd = "%s -DgpuRequired='%d'" % (cmd, cluster_config["worker"]["gpu"])
    return cmd


def _get_project_role_name(project):
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


def _max_compute_table_url(table):
    parts = table.split(".")
    if len(parts) != 2:
        raise SQLFlowDiagnostic("odps table: %s should be format db.table" %
                                table)
    return "odps://%s/tables/%s" % (parts[0], parts[1])
