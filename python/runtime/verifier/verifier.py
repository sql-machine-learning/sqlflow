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


def fetch_samples(conn, query, n=1):
    '''
    Fetch n sample(s) at most according to the query statement.

    Args:
        conn: the connection object.
        query (str): the select SQL statement.
        n (int): the maximum sample number to query. Query all samples
            if n < 0.

    Returns:
        A generator which yields each row of the data.
    '''

    query = db.limit_select(query, n)
    gen = db.db_generator(conn, query)

    # Note: Only when the iteration begins, we can get
    # gen.field_names and gen.field_types. So we take
    # the first element in the generator first, and
    # set field_names and field_types to the returned
    # result.
    gen_iter = iter(gen())
    rows = next(gen_iter, None)

    if rows is None:
        # No fetch data, just return None
        return None

    def reader():
        r = rows
        while r is not None:
            # r = (row_data, label_data), and label_data is None here
            yield r[0]
            r = next(gen_iter, None)

    reader.field_names = gen.field_names
    reader.field_types = gen.field_types
    return reader


def verify_column_name_and_type(conn, train_select, pred_select, label):
    '''
    Verify whether the columns in the SQL statement for prediction
    contain all columns except the label column in the SQL statement for
    training. This method would also verify whether the field type are
    the same for the same columns between the SQL statement for training
    and prediction.

    Args:
        conn: the connection object.
        train_select (str): the select SQL statement for training.
        pred_select (str): the select SQL statement for prediction.
        label (str): the label name for training.

    Returns:
        None

    Raises:
        ValueError: if any column name or type does not match.
    '''
    train_schema = db.selected_columns_and_types(conn, train_select)
    pred_schema = dict(db.selected_columns_and_types(conn, pred_select))

    for name, train_type in train_schema:
        if name == label:
            continue

        pred_type = pred_schema.get(name)
        if pred_type is None:
            raise ValueError(
                "the predict statement doesn't contain column %s" % name)

        if pred_type != train_type:
            raise ValueError(
                "field %s type dismatch %s(predict) vs %s(train)" %
                (name, pred_type, train_type))
