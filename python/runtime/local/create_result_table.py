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

from runtime import db
from runtime.feature.field_desc import DataType


def create_predict_table(conn, select, result_table, train_label_desc,
                         pred_label_name):
    """
    Create the result prediction table.

    Args:
        conn: the database connection object.
        select (str): the input data to predict.
        result_table (str): the output data table.
        train_label_desc (FieldDesc): the FieldDesc of the trained label.
        pred_label_name (str): the output label name to predict.

    Returns:
        A tuple of (result_column_names, train_label_index).
    """
    name_and_types = db.selected_columns_and_types(conn, select)
    train_label_index = -1
    for i, (name, _) in enumerate(name_and_types):
        if name == train_label_desc.name:
            train_label_index = i
            break

    if train_label_index >= 0:
        del name_and_types[train_label_index]

    column_strs = []
    for name, typ in name_and_types:
        column_strs.append("%s %s" %
                           (name, db.to_db_field_type(conn.driver, typ)))

    train_label_field_type = DataType.to_db_field_type(conn.driver,
                                                       train_label_desc.dtype)
    column_strs.append("%s %s" % (pred_label_name, train_label_field_type))

    drop_sql = "DROP TABLE IF EXISTS %s;" % result_table
    create_sql = "CREATE TABLE %s (%s);" % (result_table,
                                            ",".join(column_strs))
    conn.execute(drop_sql)
    conn.execute(create_sql)
    result_column_names = [item[0] for item in name_and_types]
    result_column_names.append(pred_label_name)
    return result_column_names, train_label_index
