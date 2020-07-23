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
from runtime.optimize.model_generation import \
    generate_objective_and_constraint_expression
from runtime.oss import get_bucket

__all__ = [
    'run_optimize_on_optflow',
]

OPTFLOW_HTTP_HEADERS = {
    'content-type': 'application/json',
    'accept': 'application/json',
}


def query_optflow_job_status(url, record_id, user_number, token):
    """
    Query OptFlow job status.

    Args:
        url: the URL to query job status.
        record_id: the job id.
        user_number: the user id.
        token: the OptFlow API token.

    Returns:
        A string that indicates the job status. It may be
        "success", "fail", "running", etc.
    """
    url = "{}?userNumber={}&recordId={}&token={}".format(
        url, user_number, record_id, token)
    response = requests.get(url, headers=OPTFLOW_HTTP_HEADERS)
    response.raise_for_status()
    response_json = response.json()
    if not response_json['success']:
        raise ValueError('cannot get status of job {}'.format(record_id))

    return response_json['data']['status'].lower()


def query_optflow_job_log(url, record_id, user_number, token, start_line_num):
    """
    Query OptFlow job log.

    Args:
        url: the URL to query job log.
        record_id: the job id.
        user_number: the user id.
        token: the OptFlow API token.
        start_line_num: the start line number of the logs.

    Returns:
        A tuple of (logs, end_line_num), where logs are the queried results, and
        end_line_num is the line number of the last queried logs.
    """
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
    """
    Print the OptFlow job log till the job finishes.

    Args:
        status_url: the URL to query job status.
        log_url: the URL to query job log.
        record_id: the job id.
        user_number: the user id.
        token: the OptFlow API token.

    Returns:
        Bool, whether the job is successful.
    """
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
    """
    Submit the OptFlow job.

    Args:
        train_table (str): the source table name.
        result_table (str): the table name to save the solved results.
        fsl_file_content (str): the FSL file content to submit.
        solver (str): the solver used to solve the model.
        user_number (str): the user id.

    Returns:
        None
    """
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


def run_optimize_on_optflow(train_table, columns, variables, variable_type,
                            result_value_name, objective, direction,
                            constraints, solver, result_table, user_number):
    """
    Run the optimize case in the local mode.

    Args:
        train_table (str): the source table name.
        columns (list[str]): the column names of the source table.
        variables (list[str]): the variable names to be optimized.
        variable_type (str): the variable type.
        result_value_name (str): the result value name to be optimized.
        objective (list[str]): the objective string token list.
        direction (str): "maximize" or "minimize".
        constraints (dict): the constraint expression containing the token list and GROUP BY column name.
        solver (str): the solver used to solve the model.
        result_table (str): the table name to save the solved results.
        user_number (str): the user id.

    Returns:
        None
    """

    if direction.lower() == "maximize":
        direction = "max"
    elif direction.lower() == "minimize":
        direction = "min"
    else:
        raise ValueError("direction must be maximize or minimize")

    obj_expr, c_exprs = generate_objective_and_constraint_expression(
        columns=columns,
        objective=objective,
        constraints=constraints,
        variables=variables,
        result_value_name=result_value_name,
        variable_str="@X",
        data_str="@input")

    constraint_expressions = []
    for expr, for_range, iter_vars in c_exprs:
        if for_range:
            c_expr_str = "for %s in %s: %s" % (",".join(iter_vars), for_range,
                                               expr)
        else:
            c_expr_str = expr

        constraint_expressions.append(c_expr_str)

    fsl_file_content = '''
variables: {}

var_type: {}

objective: {}
{}

constraints:
{}
'''.format(",".join(variables), variable_type, direction, obj_expr,
           "\n".join(constraint_expressions))

    submit_optflow_job(train_table=train_table,
                       result_table=result_table,
                       fsl_file_content=fsl_file_content,
                       solver=solver,
                       user_number=user_number)
