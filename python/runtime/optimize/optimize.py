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
import io
import sys

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


def contains_aggregation_function(expression):
    for expr in expression:
        if expr.lower() in AGGREGATION_FUNCTIONS:
            return True

    return False


def generate_range_constraint_func(expression, data_frame, variables,
                                   result_value_name):
    variables = [v.lower() for v in variables]
    result_value_name = result_value_name.lower()

    param_columns = {}
    for c in data_frame.columns:
        if c.lower() in variables or c.lower() == result_value_name:
            continue
        param_columns[c.lower()] = c

    result_exprs = []

    for i, expr in enumerate(expression):
        if expr.lower() == result_value_name:
            result_exprs.append("model.x[i]")
        elif expr.lower() in variables:
            if len(variables) > 1:
                raise ValueError(
                    "Invalid expression, variable {} should not be inside non aggregation constraint expression"
                    .format(expr))
            else:
                result_exprs.append("model.x[i]")
        elif expr.lower() in param_columns:
            result_exprs.append("DATA_FRAME.{}[i]".format(expr.lower()))
        else:
            result_exprs.append(expr)

    result_func_str = "".join(result_exprs)
    result_func_str = "lambda model, i: {}".format(result_func_str)
    result_func = eval(result_func_str)
    setattr(result_func, "code", result_func_str)  # for debug and unittest
    return result_func


def generate_objective_or_constraint_func(expression,
                                          data_frame,
                                          variables,
                                          result_value_name,
                                          index=None):
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

    def append_non_aggregation_expr(expr, result_exprs):
        if expr.lower() in AGGREGATION_FUNCTIONS:
            result_exprs.append(expr.lower())
        elif expr.lower() in variables:
            result_exprs.append("model.x[{}]".format(
                variables.index(expr.lower())))
        elif expr.lower() == result_value_name:
            raise ValueError(
                "Invalid expression, result value {} should not be inside non-aggregation expression"
                .format(expr))
        elif expr.lower() in param_columns:
            if index is None:
                raise ValueError(
                    "Invalid expression, param column {} should only occur constraint clause using GROUP BY"
                    .format(expr))
            else:  # TODO(sneaxiy): need check whether the value is unique
                value_column = data_frame[param_columns.get(
                    expr.lower())].to_numpy()
                value = value_column[index[0]]
                result_exprs.append(str(value))
        else:
            result_exprs.append(expr)

    result_exprs = []
    i = 0
    while i < len(expression):
        bracket_indices, next_idx = find_matched_aggregation_brackets(
            expression, i)
        assert bracket_indices is not None, "brackets not match"

        if not bracket_indices:  # no bracket
            for idx in six.moves.range(i, next_idx):
                append_non_aggregation_expr(expression[idx], result_exprs)
            i = next_idx
            continue

        left_indices = [idx[0] for idx in bracket_indices]
        right_indices = [idx[1] for idx in bracket_indices]
        left_idx, right_idx = left_indices[0], right_indices[0]

        for idx in six.moves.range(i, left_idx):
            append_non_aggregation_expr(expression[idx], result_exprs)

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
            if expression[idx] == "(":
                result_exprs.append(expression[idx])
                if idx in left_indices:
                    result_exprs.append("[")
                continue
            elif expression[idx] == ")":
                if idx in right_indices:
                    result_exprs.append(' ')
                    if index is not None:
                        result_exprs.append('for {} in {}'.format(
                            index_str, index))
                    else:
                        result_exprs.append(
                            'for {} in model.x'.format(index_str))
                    result_exprs.append(']')
                result_exprs.append(expression[idx])
                continue

            if expression[idx].lower() in AGGREGATION_FUNCTIONS:
                result_exprs.append(expression[idx].lower())
            elif expression[idx].lower() in param_columns:
                column_name = param_columns.get(expression[idx].lower())
                expr = 'DATA_FRAME.{}[{}]'.format(column_name, index_str)
                result_exprs.append(expr)
            elif expression[idx].lower() == result_value_name or (
                    len(variables) == 1
                    and expression[idx].lower() == variables[0]):
                expr = 'model.x[{}]'.format(index_str)
                result_exprs.append(expr)
            elif expression[idx].lower() in variables:
                raise ValueError(
                    "Invalid expression, variable {} should not be inside aggregation expression"
                    .format(expression[idx]))
            else:
                result_exprs.append(expression[idx])

        for idx in six.moves.range(right_idx + 1, next_idx):
            append_non_aggregation_expr(expression[idx], result_exprs)

        i = next_idx

    result_expresion = "".join(result_exprs)
    result_func_str = "lambda model: {}".format(result_expresion)
    result_func = eval(result_func_str)
    setattr(result_func, "code", result_func_str)  # for debug and unittest
    return result_func


def generate_model_with_data_frame(data_frame, variables, variable_type,
                                   result_value_name, objective, direction,
                                   constraints):
    global DATA_FRAME
    DATA_FRAME = data_frame

    model = pyomo_env.ConcreteModel()
    var_num = len(data_frame)
    model.x = pyomo_env.Var(list(range(var_num)), within=eval(variable_type))

    objective_func = generate_objective_or_constraint_func(
        expression=objective,
        data_frame=data_frame,
        variables=variables,
        result_value_name=result_value_name)

    model.objective = pyomo_env.Objective(rule=objective_func,
                                          sense=eval(direction))

    attr_index = 0
    for i, c in enumerate(constraints):
        expression = c.get("expression")
        group_by = c.get("group_by")
        has_aggregation_func = contains_aggregation_function(expression)

        if group_by:
            group_by_column = None

            for column in data_frame.columns:
                if group_by.lower() == column.lower():
                    group_by_column = column
                    break

            if group_by_column is None:
                raise ValueError(
                    "Cannot find GROUP BY column {}".format(group_by))

            values = np.unique(data_frame[group_by_column].to_numpy()).tolist()
            for v in values:
                index = np.where(data_frame[group_by_column] == v)[0].tolist()
                if has_aggregation_func:
                    constraint_func = generate_objective_or_constraint_func(
                        expression=expression,
                        data_frame=data_frame,
                        variables=variables,
                        result_value_name=result_value_name,
                        index=index)
                    constraint = pyomo_env.Constraint(rule=constraint_func)
                else:
                    constraint_func = generate_range_constraint_func(
                        expression=expression,
                        data_frame=data_frame,
                        variables=variables,
                        result_value_name=result_value_name)
                    index_set = pyomo_env.Set(initialize=index)
                    constraint = pyomo_env.Constraint(index_set,
                                                      rule=constraint_func)

                attr_name = "c_{}".format(attr_index)
                setattr(model, attr_name, constraint)
                attr_index += 1
        else:
            if has_aggregation_func:
                constraint_func = generate_objective_or_constraint_func(
                    expression=expression,
                    data_frame=data_frame,
                    variables=variables,
                    result_value_name=result_value_name)
                constraint = pyomo_env.Constraint(rule=constraint_func)
            else:
                constraint_func = generate_range_constraint_func(
                    expression=expression,
                    data_frame=data_frame,
                    variables=variables,
                    result_value_name=result_value_name)
                range_set = pyomo_env.RangeSet(0, var_num - 1)
                constraint = pyomo_env.Constraint(range_set,
                                                  rule=constraint_func)

            attr_name = "c_{}".format(attr_index)
            setattr(model, attr_name, constraint)
            attr_index += 1

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


def run_optimize(datasource, select, variables, variable_type,
                 result_value_name, objective, direction, constraints, solver,
                 result_table):
    data_frame = load_db_data_to_data_frame(datasource, select)
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
