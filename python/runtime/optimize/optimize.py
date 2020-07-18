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

import numpy as np
import pandas as pd
import pyomo.environ as pyomo_env
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

    result_expression = "".join(result_tokens)
    return result_expression


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


def load_db_data_to_data_frame(datasource,
                               select=None,
                               odps_table=None,
                               load_schema_only=False):
    if odps_table is None:
        conn = db.connect_with_data_source(datasource)
        selected_cols = db.selected_cols(conn, select)
        if load_schema_only:
            return pd.DataFrame(columns=selected_cols)

        generator = db.db_generator(conn, select)
    else:
        project, table = odps_table.split('.')
        conn = db.connect_with_data_source(datasource)
        schema = conn.get_table(table).schema
        selected_cols = [column.name for column in schema]
        if load_schema_only:
            return pd.DataFrame(columns=selected_cols)

        select_sql = "SELECT * FROM {}".format(table)
        instance = conn.execute_sql(select_sql)

        if not instance.is_successful():
            raise ValueError('cannot get data from table {}.{}'.format(
                project, table))

        def generator_func():
            from odps import tunnel
            compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
            with instance.open_reader(tunnel=False,
                                      compress=compress) as reader:
                for record in reader:
                    row_value = [
                        record[i] for i in six.moves.range(len(selected_cols))
                    ]
                    yield row_value, None

        generator = generator_func

    dtypes = [None] * len(selected_cols)
    values = [[] for _ in six.moves.range(len(selected_cols))]
    for row_value, _ in generator():
        for i, item in enumerate(row_value):
            if dtypes[i] == np.str:
                values[i].append(item)
                continue

            float_value = None
            try:
                float_value = float(item)
            except:
                pass

            if float_value is None:  # cannot convert to float value
                dtypes[i] = np.str
            else:
                item = float_value
                int_value = long(item) if six.PY2 else int(item)
                if int_value != item:
                    dtypes[i] = np.float64

            values[i].append(item)

    numpy_dict = collections.OrderedDict()
    for col_name, dtype, value in six.moves.zip(selected_cols, dtypes, values):
        if dtype is None:
            dtype = np.int64

        numpy_dict[col_name] = np.array(value, dtype=dtype)

    df = pd.DataFrame(data=numpy_dict)
    return df


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
