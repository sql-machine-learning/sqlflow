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

import threading

import numpy as np
import pandas as pd
import pyomo.environ as pyomo_env
import runtime.db as db
import runtime.verifier as verifier
import six
from pyomo.environ import (Integers, NegativeIntegers, NegativeReals,
                           NonNegativeIntegers, NonNegativeReals,
                           NonPositiveIntegers, NonPositiveReals,
                           PositiveIntegers, PositiveReals, Reals, maximize,
                           minimize)
from runtime.optimize.model_generation import (
    generate_objective_and_constraint_expression,
    generate_unique_result_value_name)

# FIXME(sneaxiy): do not know why Pyomo requires that the data frame must be
# a global variable
DATA_FRAME = None
DATA_FRAME_LOCK = threading.Lock()


def generate_model_with_data_frame(data_frame, variables, variable_type,
                                   result_value_name, objective, direction,
                                   constraints):
    direction = direction.lower()
    if direction not in ['maximize', 'minimize']:
        raise ValueError("direction must be one of 'maximize' or 'minimize'")

    model = pyomo_env.ConcreteModel()
    var_num = len(data_frame)
    model.x = pyomo_env.Var(list(range(var_num)), within=eval(variable_type))

    columns = data_frame.columns

    variable_str = "model.x"
    data_str = "DATA_FRAME"

    obj_expr, c_exprs = generate_objective_and_constraint_expression(
        columns=columns,
        objective=objective,
        constraints=constraints,
        variables=variables,
        result_value_name=result_value_name,
        variable_str=variable_str,
        data_str=data_str)

    DATA_FRAME_LOCK.acquire()
    try:
        global DATA_FRAME
        DATA_FRAME = data_frame
        obj_func = eval("lambda model: %s" % obj_expr)
        model.objective = pyomo_env.Objective(rule=obj_func,
                                              sense=eval(direction))

        for i, (expr, for_range, iter_vars) in enumerate(c_exprs):
            attr_name = "constraint_%d" % i

            if for_range:
                assert iter_vars, "for_range and iter_vars must be both non-empty"
                setattr(model, attr_name, pyomo_env.ConstraintList())
                constraint_list = getattr(model, attr_name)
                add_constraint_str = "lambda model, constraint_list: [constraint_list.add(%s) for %s in %s]" % (
                    expr, ",".join(iter_vars), for_range)
                eval(add_constraint_str)(model, constraint_list)
            else:
                assert not iter_vars, "for_range and iter_vars must be both empty"
                func = eval('lambda model: %s' % expr)
                constraint = pyomo_env.Constraint(rule=func)
                setattr(model, attr_name, constraint)
    finally:
        DATA_FRAME = None
        DATA_FRAME_LOCK.release()
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
    generator = verifier.fetch_samples(conn, select, n=-1)
    names = generator.field_names
    dtypes = []
    for dtype in generator.field_types:
        if dtype in ['VARCHAR', 'CHAR', 'TEXT', 'STRING']:
            dtypes.append(np.str)
        else:
            dtypes.append(np.float64)

    df = pd.DataFrame(columns=names)
    for i, rows in enumerate(generator()):
        df.loc[i] = rows

    for name, dtype in zip(names, dtypes):
        df[name] = df[name].astype(dtype)

    conn.close()
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

    result_value_name = generate_unique_result_value_name(
        columns=data_frame.columns,
        result_value_name=result_value_name,
        variables=variables)

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
