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

import collections
import json
import os
import sys
import time

import numpy as np
import pandas as pd
import pyomo.environ as pyomo_env
import requests
import runtime.db as db
import six
from pyomo.environ import (Integers, NegativeIntegers, NegativeReals,
                           NonNegativeIntegers, NonNegativeReals,
                           NonPositiveIntegers, NonPositiveReals,
                           PositiveIntegers, PositiveReals, Reals, maximize,
                           minimize)

# TODO(sneaxiy): support more aggregation functions if needed
AGGREGATION_FUNCTIONS = ['sum']

# FIXME(sneaxiy): do not know why pyomo requires that DATA_FRAME must be a global variable
DATA_FRAME = None


def find_prev_non_blank(expression, i):
    if i >= len(expression):
        return -1

    while i >= 0:
        if len(expression[i].strip()) == 0:
            i -= 1
            continue

        return i

    return -1


def find_next_non_blank(expression, i):
    while i < len(expression):
        if len(expression[i].strip()) == 0:
            i += 1
            continue

        return i

    return -1


def find_matched_aggregation_brackets(expression, i):
    brackets = []
    left_bracket_num = 0
    while i < len(expression):
        i = find_next_non_blank(expression, i)
        if i < 0:
            break

        if expression[i] == "(":
            brackets.append([i, None, None])
            left_bracket_num += 1
        elif expression[i] == ")":
            left_bracket_num -= 1
            if left_bracket_num < 0:
                return None, None

            brackets[left_bracket_num][1] = i
            brackets[left_bracket_num][2] = left_bracket_num
            if left_bracket_num == 0:
                break

        i += 1

    aggregation_brackets = []
    for left_idx, right_idx, depth in brackets:
        j = find_prev_non_blank(expression, left_idx - 1)
        if j >= 0 and expression[j].lower() in AGGREGATION_FUNCTIONS:
            aggregation_brackets.append((left_idx, right_idx, depth))

    return aggregation_brackets, min(i + 1, len(expression))


def contains_aggregation_function(tokens):
    for token in tokens:
        if token.lower() in AGGREGATION_FUNCTIONS:
            return True

    return False


def generate_non_aggregated_constraint_expression(tokens,
                                                  data_frame,
                                                  variables,
                                                  result_value_name,
                                                  variable_str="model.x",
                                                  data_frame_str="DATA_FRAME"):
    variables = [v.lower() for v in variables]
    result_value_name = result_value_name.lower()

    param_columns = {}
    for c in data_frame.columns:
        if c.lower() in variables or c.lower() == result_value_name:
            continue
        param_columns[c.lower()] = c

    result_tokens = []

    for i, token in enumerate(tokens):
        if token.lower() == result_value_name:
            result_tokens.append("{}[i]".format(variable_str))
        elif token.lower() in variables:
            if len(variables) > 1:
                raise ValueError(
                    "Invalid expression, variable {} should not be inside non aggregation constraint expression"
                    .format(token))
            else:
                result_tokens.append("{}[i]".format(variable_str))
        elif token.lower() in param_columns:
            column_name = param_columns.get(token.lower())
            result_tokens.append('{}["{}"][i]'.format(data_frame_str,
                                                      column_name))
        else:
            result_tokens.append(token)

    result_expression = "".join(result_tokens)
    return result_expression


def generate_objective_or_aggregated_constraint_expression(
    tokens,
    data_frame,
    variables,
    result_value_name,
    indices=None,
    variable_str="model.x",
    data_frame_str="DATA_FRAME"):
    variables = [v.lower() for v in variables]
    result_value_name = result_value_name.lower()

    param_columns = {}
    variable_columns = {}
    for c in data_frame.columns:
        if c.lower() in variables:
            variable_columns[c.lower()] = c
            continue

        if c.lower() == result_value_name:
            continue

        param_columns[c.lower()] = c

    def append_non_aggregation_token(token, result_tokens):
        if token.lower() in AGGREGATION_FUNCTIONS:
            result_tokens.append(token.lower())
        elif token.lower() in variables:
            result_tokens.append("{}[{}]".format(
                variable_str, variables.index(token.lower())))
        elif token.lower() == result_value_name:
            raise ValueError(
                "Invalid expression, result value {} should not be inside non-aggregation expression"
                .format(token))
        elif token.lower() in param_columns:
            if indices is None:
                raise ValueError(
                    "Invalid expression, param column {} should only occur constraint clause using GROUP BY"
                    .format(token))
            else:  # TODO(sneaxiy): need check whether the value is unique
                value_column = data_frame[param_columns.get(
                    token.lower())].to_numpy()
                value = value_column[indices[0]]
                result_tokens.append(str(value))
        else:
            result_tokens.append(token)

    result_tokens = []
    i = 0
    while i < len(tokens):
        bracket_indices, next_idx = find_matched_aggregation_brackets(
            tokens, i)
        assert bracket_indices is not None, "brackets not match"

        if not bracket_indices:  # no bracket
            for idx in six.moves.range(i, next_idx):
                append_non_aggregation_token(tokens[idx], result_tokens)
            i = next_idx
            continue

        left_indices = [idx[0] for idx in bracket_indices]
        right_indices = [idx[1] for idx in bracket_indices]
        left_idx, right_idx = left_indices[0], right_indices[0]

        for idx in six.moves.range(i, left_idx):
            append_non_aggregation_token(tokens[idx], result_tokens)

        def get_depth(idx):
            max_depth_idx = -1
            k = 0
            for l, r, d in bracket_indices:
                if idx < l or idx > r:
                    continue

                if max_depth_idx < 0 or bracket_indices[max_depth_idx][2] < d:
                    max_depth_idx = k

                k += 1

            if max_depth_idx < 0:
                raise ValueError("Cannot find depth of bracket")

            return bracket_indices[max_depth_idx][2]

        for idx in six.moves.range(left_idx, right_idx + 1):
            depth = get_depth(idx)
            index_str = 'i_{}'.format(depth)
            if tokens[idx] == "(":
                result_tokens.append(tokens[idx])
                if idx in left_indices:
                    result_tokens.append("[")
                continue
            elif tokens[idx] == ")":
                if idx in right_indices:
                    result_tokens.append(' ')
                    if indices is not None:
                        result_tokens.append('for {} in {}'.format(
                            index_str, indices))
                    else:
                        result_tokens.append('for {} in {}'.format(
                            index_str, variable_str))
                    result_tokens.append(']')
                result_tokens.append(tokens[idx])
                continue

            if tokens[idx].lower() in AGGREGATION_FUNCTIONS:
                result_tokens.append(tokens[idx].lower())
            elif tokens[idx].lower() in param_columns:
                column_name = param_columns.get(tokens[idx].lower())
                expr = '{}["{}"][{}]'.format(data_frame_str, column_name,
                                             index_str)
                result_tokens.append(expr)
            elif tokens[idx].lower() == result_value_name or (
                    len(variables) == 1
                    and tokens[idx].lower() == variables[0]):
                expr = '{}[{}]'.format(variable_str, index_str)
                result_tokens.append(expr)
            elif tokens[idx].lower() in variables:
                raise ValueError(
                    "Invalid expression, variable {} should not be inside aggregation expression"
                    .format(tokens[idx]))
            else:
                result_tokens.append(tokens[idx])

        for idx in six.moves.range(right_idx + 1, next_idx):
            append_non_aggregation_token(tokens[idx], result_tokens)

        i = next_idx

    result_expresion = "".join(result_tokens)
    return result_expresion


def generate_objective_or_constraint_expressions(tokens,
                                                 data_frame,
                                                 variables,
                                                 result_value_name,
                                                 group_by="",
                                                 variable_str="model.x",
                                                 data_frame_str="DATA_FRAME"):
    has_aggregation_func = contains_aggregation_function(tokens)
    result_expressions = []

    if group_by:
        assert has_aggregation_func, "GROUP BY must be used with aggregation functions"
        group_by_column = None

        for column in data_frame.columns:
            if group_by.lower() == column.lower():
                group_by_column = column
                break

        if group_by_column is None:
            raise ValueError("Cannot find GROUP BY column {}".format(group_by))

        values = np.unique(data_frame[group_by_column].to_numpy()).tolist()
        for v in values:
            indices = np.where(data_frame[group_by_column] == v)[0].tolist()
            expression = generate_objective_or_aggregated_constraint_expression(
                tokens=tokens,
                data_frame=data_frame,
                variables=variables,
                result_value_name=result_value_name,
                indices=indices,
                variable_str=variable_str,
                data_frame_str=data_frame_str)
            result_expressions.append((expression, ))
    else:
        if has_aggregation_func:
            expression = generate_objective_or_aggregated_constraint_expression(
                tokens=tokens,
                data_frame=data_frame,
                variables=variables,
                result_value_name=result_value_name,
                variable_str=variable_str,
                data_frame_str=data_frame_str)
            result_expressions.append((expression, ))
        else:
            expression = generate_non_aggregated_constraint_expression(
                tokens=tokens,
                data_frame=data_frame,
                variables=variables,
                result_value_name=result_value_name,
                variable_str=variable_str,
                data_frame_str=data_frame_str)
            result_expressions.append(
                (expression, None))  # None means all variables

    return result_expressions


def generate_model_with_data_frame(data_frame, variables, variable_type,
                                   result_value_name, objective, direction,
                                   constraints):
    global DATA_FRAME
    DATA_FRAME = data_frame

    model = pyomo_env.ConcreteModel()
    var_num = len(data_frame)
    model.x = pyomo_env.Var(list(range(var_num)), within=eval(variable_type))

    objective_expression = generate_objective_or_constraint_expressions(
        tokens=objective,
        data_frame=data_frame,
        variables=variables,
        result_value_name=result_value_name)
    assert len(objective_expression) == 1 and len(objective_expression[0]) == 1, \
        "there must be only one objective expression"
    objective_func = eval("lambda model: {}".format(
        objective_expression[0][0]))
    model.objective = pyomo_env.Objective(rule=objective_func,
                                          sense=eval(direction))

    attr_index = 0
    for c in constraints:
        tokens = c.get("tokens")
        group_by = c.get("group_by")

        expressions = generate_objective_or_constraint_expressions(
            tokens=tokens,
            data_frame=data_frame,
            variables=variables,
            result_value_name=result_value_name,
            group_by=group_by)

        for expr in expressions:
            attr_name = 'c_{}'.format(attr_index)
            attr_index += 1

            if len(expr) == 1:  # (expression, )
                func = eval('lambda model: {}'.format(expr[0]))
                setattr(model, attr_name, pyomo_env.Constraint(rule=func))
            else:  # (expression, range), where range may be None and None means all variables
                indices = expr[1]
                if indices is None:
                    indices = list(six.moves.range(var_num))
                func = eval('lambda model, i: {}'.format(expr[0]))
                index_set = pyomo_env.Set(initialize=indices)
                setattr(model, attr_name,
                        pyomo_env.Constraint(index_set, rule=func))

    DATA_FRAME = None
    return model


def solve_model(model, solver):
    opt = pyomo_env.SolverFactory(solver)
    solved_results = opt.solve(model)

    result_values = []
    has_error = False
    pyomo_dtype = None

    for idx in model.x:
        value = model.x[idx](exception=False)
        # If any variable is not initialized,
        # the solving process fails.
        if value is None:
            has_error = True
            break
        else:
            result_values.append(value)

        if pyomo_dtype is None:
            pyomo_dtype = type(model.x[idx])

        assert pyomo_dtype == type(
            model.x[idx]), "all variables must be of the same data type"

    if has_error:
        msg = 'Solve model error. Termination condition: {}.'\
            .format(solved_results.solver.termination_condition)
        raise ValueError(msg)

    np_dtype = np.int64 if model.x[0].is_integer() else np.float64
    return np.array(result_values, dtype=np_dtype)


def load_odps_table_to_data_frame(odps_table, load_schema_only=False):
    from odps import ODPS
    from odps.df import DataFrame
    project, table = odps_table.split('.')
    ak = os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
    sk = os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
    endpoint = os.environ.get("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
    if not endpoint.startswith('http://') and not endpoint.startswith(
            'https://'):
        endpoint = "https://" + endpoint

    endpoint = endpoint.split('?')[0]
    odps_conf = ODPS(access_id=ak,
                     secret_access_key=sk,
                     project=project,
                     endpoint=endpoint)
    table = odps_conf.get_table(table)
    if load_schema_only:
        columns = [column.name for column in table.schema]
        pandas_df = pd.DataFrame(columns=columns)
    else:
        odps_df = DataFrame(table)
        # NOTE(sneaxiy): to_pandas is extremely slow. Need a better way to speedup the code
        pandas_df = odps_df.to_pandas()

    return pandas_df


def load_db_data_to_data_frame(datasource, select):
    conn = db.connect_with_data_source(datasource)
    selected_cols = db.selected_cols(conn.driver, conn, select)
    generator = db.db_generator(conn.driver, conn, select)

    dtypes = [None] * len(selected_cols)
    values = [[] for _ in six.moves.range(len(selected_cols))]
    for row_value, _ in generator():
        for i, item in enumerate(row_value):
            if isinstance(item, six.string_types):
                dtypes[i] = np.str

            if dtypes[i] != np.str:
                if isinstance(item, (six.integer_types, float)):
                    int_val = long(item) if six.PY2 else int(item)
                    if int_val != item:
                        dtypes[i] = np.float64
                else:
                    raise ValueError("unsupported data type {}".format(
                        type(item)))

            values[i].append(item)

    numpy_dict = collections.OrderedDict()
    for col_name, dtype, value in six.moves.zip(selected_cols, dtypes, values):
        if dtype is None:
            dtype = np.int64

        numpy_dict[col_name] = np.array(value, dtype=dtype)

    return pd.DataFrame(numpy_dict)


def save_solved_result_in_db(solved_result, data_frame, variables,
                             result_value_name, datasource, result_table):
    column_names = []
    for col in data_frame.columns:
        found = False
        for var in variables:
            if var.lower() == col.lower():
                found = True
                break

        if found:
            column_names.append(col)

    data_frame = data_frame[[*column_names]]

    if len(variables) == 1 and variables[0].lower() == result_value_name.lower(
    ):
        result_value_name += "_value"

    column_names.append(result_value_name)
    data_frame[result_value_name] = solved_result

    conn = db.connect_with_data_source(datasource)
    with db.buffered_db_writer(conn.driver, conn, result_table,
                               column_names) as w:
        for i in six.moves.range(len(data_frame)):
            rows = list(data_frame.loc[i])
            w.write(rows)

    print('Solved result is:')
    print(data_frame)
    print('Saved in {}.'.format(result_table))


def run_optimize_locally(datasource, select, variables, variable_type,
                         result_value_name, objective, direction, constraints,
                         solver, result_table):
    data_frame = load_db_data_to_data_frame(datasource=datasource,
                                            select=select)
    model = generate_model_with_data_frame(data_frame=data_frame,
                                           variables=variables,
                                           variable_type=variable_type,
                                           result_value_name=result_value_name,
                                           objective=objective,
                                           direction=direction,
                                           constraints=constraints)
    solved_result = solve_model(model, solver)
    save_solved_result_in_db(solved_result=solved_result,
                             data_frame=data_frame,
                             variables=variables,
                             result_value_name=result_value_name,
                             datasource=datasource,
                             result_table=result_table)


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


def send_optflow_http_request(train_table, result_table, fsl_file_content,
                              solver, user_number):
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

    import runtime.pai.utils as utils
    import uuid
    import oss2

    bucket_name = "sqlflow-optflow-models"
    bucket = utils.get_bucket(bucket_name)
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
                print("Job succeeds, saved in {}.".format(result_table))
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


def run_optimize_on_optflow(train_table, variables, variable_type,
                            result_value_name, objective, direction,
                            constraints, solver, result_table, user_number):
    variable_str = "@X"
    data_frame_str = "@input"

    if direction.lower() == "maximize":
        direction = "max"
    elif direction.lower() == "minimize":
        direction = "min"
    else:
        raise ValueError("direction must be maximize or minimize")

    # Need to load the table data only when there is any GROUP BY clause.
    load_schema_only = True
    for c in constraints:
        if c["group_by"]:
            assert contains_aggregation_function(c["tokens"]), \
                "GROUP BY must be used with aggregation functions"
            load_schema_only = False

    data_frame = load_odps_table_to_data_frame(
        odps_table=train_table, load_schema_only=load_schema_only)
    objective_expression = generate_objective_or_constraint_expressions(
        tokens=objective,
        data_frame=data_frame,
        variables=variables,
        result_value_name=result_value_name,
        variable_str=variable_str,
        data_frame_str=data_frame_str)
    assert len(objective_expression) == 1 and len(objective_expression[0]) == 1, \
        "there must be only one objective expression"
    objective_expression = objective_expression[0][0]

    constraint_expressions = []
    for c in constraints:
        tokens = c.get("tokens")
        group_by = c.get("group_by")

        expressions = generate_objective_or_constraint_expressions(
            tokens=tokens,
            data_frame=data_frame,
            variables=variables,
            result_value_name=result_value_name,
            group_by=group_by,
            variable_str=variable_str,
            data_frame_str=data_frame_str)

        for expr in expressions:
            if len(expr) == 1:  # (expression, )
                expr = expr[0]
            else:  # (expression, range), where range may be None and None means all variables
                range_expr = expr[1]
                if range_expr is None:
                    range_expr = variable_str
                expr = "for i in {}: {}".format(range_expr, expr[0])

            constraint_expressions.append(expr)

    fsl_file_content = '''
variables: {}

var_type: {}

objective: {}
{}

constraints:
{}
'''.format(",".join(variables), variable_type, direction, objective_expression,
           "\n".join(constraint_expressions))
    send_optflow_http_request(train_table=train_table,
                              result_table=result_table,
                              fsl_file_content=fsl_file_content,
                              solver=solver,
                              user_number=user_number)
