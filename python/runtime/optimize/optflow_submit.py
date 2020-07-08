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
import sys
import time
import uuid

import oss2
import requests
import six
from runtime.pai.oss import get_bucket

__all__ = [
    'submit_optflow_job',
]

OPTFLOW_HTTP_HEADERS = {
    'content-type': 'application/json',
    'accept': 'application/json',
}


def query_optflow_job_status(url, record_id, user_number, token):
    url = "{}?userNumber={}&recordId={}&token={}".format(
        url, user_number, record_id, token)
    response = requests.get(url, headers=OPTFLOW_HTTP_HEADERS)
    response.raise_for_status()
    response_json = response.json()
    if not response_json['success']:
        raise ValueError('cannot get status of job {}'.format(record_id))

    return response_json['data']['status'].lower()


def query_optflow_job_log(url, record_id, user_number, token, start_line_num):
    url = "{}?userNumber={}&recordId={}&token={}".format(
        url, user_number, record_id, token)
    response = requests.get(url, headers=OPTFLOW_HTTP_HEADERS, stream=True)
    response.raise_for_status()
    response_json = response.json()
    if not response_json['success']:
        raise ValueError('cannot get log of job {}'.format(record_id))

    logs = response_json['data']['logs']
    end_line_num = len(logs)

    # NOTE(sneaxiy): ascii(log) is necessary because the character inside
    # log may be out of the range of ASCII characters.
    # The slice [1:-1] is used to remove the quotes. e.g.:
    # original string "abc" -> ascii("abc") outputs "'abc'"
    # -> the slice [1:-1] outputs "abc"
    logs = [ascii(log)[1:-1] for log in logs[start_line_num:]]
    return logs, end_line_num


def print_job_log_till_finish(status_url, log_url, record_id, user_number,
                              token):
    def call_func_with_retry(func, times):
        for _ in six.moves.range(times - 1):
            try:
                return func()
            except:
                pass

        return func()

    status = None
    line_num = 0
    while True:
        query_status = lambda: query_optflow_job_status(
            status_url, record_id, user_number, token)
        query_log = lambda: query_optflow_job_log(log_url, record_id,
                                                  user_number, token, line_num)
        status = call_func_with_retry(query_status, 3)
        logs, line_num = call_func_with_retry(query_log, 3)

        for log in logs:
            print(log)

        # status may be 'success', 'failed', 'running', 'prepare'
        if status in ['success', 'failed']:
            break

        time.sleep(2)  # sleep for some times

    return status == 'success'


def submit_optflow_job(train_table, result_table, fsl_file_content, solver,
                       user_number):
    project_name = train_table.split(".")[0]

    snapshot_id = os.getenv("SQLFLOW_OPTFLOW_SNAPSHOT_ID")
    if not snapshot_id:
        raise ValueError("SQLFLOW_OPTFLOW_SNAPSHOT_ID must be set")

    token = os.getenv("SQLFLOW_OPTFLOW_TOKEN")
    if not token:
        raise ValueError("SQLFLOW_OPTFLOW_TOKEN must be set")

    submit_job_url = os.getenv("SQLFLOW_OPTFLOW_SUBMIT_JOB_URL")
    if not submit_job_url:
        raise ValueError("SQLFLOW_OPTFLOW_SUBMIT_JOB_URL must be set")

    query_job_status_url = os.getenv("SQLFLOW_OPTFLOW_QUERY_JOB_STATUS_URL")
    if not query_job_status_url:
        raise ValueError("SQLFLOW_OPTFLOW_QUERY_JOB_STATUS_URL must be set")

    query_job_log_url = os.getenv("SQLFLOW_OPTFLOW_QUERY_JOB_LOG_URL")
    if not query_job_log_url:
        raise ValueError("SQLFLOW_OPTFLOW_QUERY_JOB_LOG_URL must be set")

    bucket_name = "sqlflow-optflow-models"
    bucket = get_bucket(bucket_name)
    try:
        bucket_info = bucket.get_bucket_info()
    except oss2.exceptions.NoSuchBucket:
        # Create bucket if not exists
        bucket.create_bucket()
        bucket_info = bucket.get_bucket_info()

    fsl_file_id = '{}.fsl'.format(uuid.uuid4())
    bucket.put_object(fsl_file_id, fsl_file_content)
    should_delete_object = True
    try:
        bucket.put_object_acl(fsl_file_id, oss2.BUCKET_ACL_PUBLIC_READ)
        fsl_url = "http://{}.{}/{}".format(bucket_name,
                                           bucket_info.extranet_endpoint,
                                           fsl_file_id)

        input_params = {
            "input_table": train_table,
            "output_table": result_table,
            "fsl_path": fsl_url,
            "solver_name": solver,
        }

        json_data = {
            "userNumber": user_number,
            "projectName": project_name,
            "snapshotId": snapshot_id,
            "token": token,
            "inputParams": input_params,
        }

        response = requests.post(submit_job_url,
                                 json=json_data,
                                 headers=OPTFLOW_HTTP_HEADERS)
        response.raise_for_status()
        response_json = response.json()
        if not response_json['success']:
            raise ValueError("Job submission fails")

        print('Job submission succeeds')
        record_id = response_json['data']['recordId']
        try:
            success = print_job_log_till_finish(query_job_status_url,
                                                query_job_log_url, record_id,
                                                user_number, token)
            if success:
                print("Job succeeds. Save solved result in {}.".format(
                    result_table))
            else:
                print("Job fails.")
        except:
            # FIXME(sneaxiy): we should not delete object if there is any
            # network error when querying job status and logs. But when
            # should we clean the object?
            should_delete_object = False
            six.reraise(*sys.exc_info())
    finally:
        if should_delete_object:
            bucket.delete_object(fsl_file_id)
