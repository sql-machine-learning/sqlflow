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
from os import path

from runtime import oss
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.pai.submitter import gen_rand_string

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
def alisa_execute(submit_code, cfg):
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
        return SQLFlowDiagnostic("Unknown AlisaTaskType %d" % task_type)


def upload_resource_and_submit_alisa_task(datasource, local_tar_file,
                                          params_file, alisa_exec_code):
    """Upload the packed resource and submit an Alisa task

    Args:
        datasource: the datasource to use
        local_tar_file: zipped local resource, including code and params
        params_file: the params file
        alisa_exec_code: the command to submit a PAI task
    """
    oss_code_obj = gen_rand_string(16)
    bucket = getAlisaBucket()
    oss_code_url = upload_resource(local_tar_file, oss_code_obj, bucket)

    # upload params.txt for additional training parameters.
    oss_params_obj = gen_rand_string(16)
    oss_params_url = upload_resource(params_file, oss_params_obj, bucket)
    conf = {
        "codeResourceURL": oss_code_url,
        "paramsResourceURL": oss_params_url,
        "resourceName": local_tar_file,
        "paramsFile": params_file,
    }
    submit_alisa_task(datasource, AlisaTaskTypePAI, alisa_exec_code, conf)

    bucket.delete_object(oss_code_obj)
    bucket.delete_object(oss_params_obj)
