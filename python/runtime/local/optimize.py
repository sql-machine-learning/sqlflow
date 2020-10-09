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

import runtime.db as db
from runtime.feature.field_desc import DataType
from runtime.optimize.local import run_optimize_locally


def _create_result_table(datasource, select, variables, result_value_name,
                         variable_type, result_table):
    if variable_type.endswith('Integers') or variable_type == "Binary":
        result_type = DataType.INT64
    elif variable_type.endswith('Reals'):
        result_type = DataType.FLOAT32
    else:
        raise ValueError("unsupported variable type %s" % variable_type)

    conn = db.connect_with_data_source(datasource)
    name_and_types = dict(db.selected_columns_and_types(conn, select))
    columns = []
    for var in variables:
        field_type = db.to_db_field_type(conn.driver, name_and_types.get(var))
        columns.append("%s %s" % (var, field_type))

    if len(variables) == 1 and variables[0].lower() == result_value_name.lower(
    ):
        result_value_name += "_value"

    columns.append("%s %s" %
                   (result_value_name,
                    DataType.to_db_field_type(conn.driver, result_type)))
    column_str = ",".join(columns)

    conn.execute("DROP TABLE IF EXISTS %s" % result_table)
    create_sql = "CREATE TABLE %s (%s)" % (result_table, column_str)
    conn.execute(create_sql)
    conn.close()


def run_optimize(datasource, select, variables, result_value_name,
                 variable_type, objective, direction, constraints, solver,
                 result_table, user_id):
    _create_result_table(datasource, select, variables, result_value_name,
                         variable_type, result_table)
    return run_optimize_locally(datasource=datasource,
                                select=select,
                                variables=variables,
                                variable_type=variable_type,
                                result_value_name=result_value_name,
                                objective=objective,
                                direction=direction,
                                constraints=constraints,
                                solver=solver,
                                result_table=result_table)
