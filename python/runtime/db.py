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

import contextlib
import re

import numpy as np
import runtime.db_writer as db_writer
import six
from runtime.dbapi import connect as dbapi_connect


def connect_with_data_source(driver_dsn):
    return dbapi_connect(driver_dsn)


INT64_TYPE = long if six.PY2 else int  # noqa: F821
# NOTE(typhoonzero): use a magic number to represent missing values for
# xgboost, since we can not write a dmatrix file with NaN.
XGBOOST_NULL_MAGIC = 9999.0


def read_feature(raw_val, feature_spec, feature_name, is_xgboost):
    # FIXME(typhoonzero): Should use correct dtype here.
    null_feature_error = ValueError(
        "column %s value is NULL, expected dense vector with delimiter %s" %
        (feature_name, feature_spec["delimiter"]))
    if feature_spec["is_sparse"]:
        if feature_spec["format"] == "kv":
            if is_xgboost and raw_val is None:
                indices = np.array([], dtype=np.int64)
                values = np.array([], dtype=np.float32)
            else:
                if feature_spec.get("delimiter_kv", "") != "":
                    delim1 = feature_spec["delimiter"]
                    delim2 = feature_spec["delimiter_kv"]
                    indices_dtype = feature_spec["dtype"]
                else:  # default libsvm kv format delimiters: "k:v k:v..."
                    delim1 = " "
                    delim2 = ":"
                    indices_dtype = "int64"
                items = raw_val.split(delim1)
                items = [item.split(delim2, 2) for item in items]
                # NOTE(typhoonzero): dtype is already checked when compiling:
                # ir_generator.go
                indices = np.array([item[0] for item in items],
                                   dtype=indices_dtype)
                if not is_xgboost:
                    # tf need sparse indices to be a column vector.
                    indices = indices.reshape(indices.size, 1)
                dtype_weight = feature_spec.get("dtype_weight", "float32")
                values = np.array([
                    float(item[1]) if len(item) == 2 else 1.0 for item in items
                ],
                                  dtype=dtype_weight)
        else:  # csv format
            if is_xgboost and raw_val is None:
                indices = np.array([], dtype=int)
                values = np.array([], dtype=np.int64)
            else:
                indices = np.fromstring(raw_val,
                                        dtype=int,
                                        sep=feature_spec["delimiter"])
                indices = indices.reshape(indices.size, 1)
                values = np.ones([indices.size], dtype=np.int64)

        dense_shape = np.array(feature_spec["shape"], dtype=np.int64)
        return indices, values, dense_shape
    elif feature_spec["delimiter"] != "":
        if feature_spec.get("delimiter_kv", "") != "":
            raise ValueError(
                "not supported DENSE column with key:value list format.")
        # Dense string vector
        if feature_spec["dtype"] == "float32":
            if raw_val is None:
                raise null_feature_error
            else:
                vec = np.fromstring(raw_val,
                                    dtype=np.float32,
                                    sep=feature_spec["delimiter"])
        elif feature_spec["dtype"] == "int64":
            if raw_val is None:
                raise null_feature_error
            else:
                vec = np.fromstring(raw_val,
                                    dtype=np.int64,
                                    sep=feature_spec["delimiter"])
        else:
            raise ValueError('unrecognize dtype {}'.format(
                feature_spec["dtype"]))

        vec = vec.reshape(list(feature_spec["shape"]))
        return vec,
    elif feature_spec["dtype"] == "float32":
        if raw_val is None:
            if is_xgboost:
                return float(XGBOOST_NULL_MAGIC),
            else:
                raise null_feature_error
        else:
            return float(raw_val),
    elif feature_spec["dtype"] == "int64":
        if raw_val is None:
            if is_xgboost:
                return int(XGBOOST_NULL_MAGIC),
            else:
                raise null_feature_error
        else:
            int_raw_val = INT64_TYPE(raw_val)
            return int_raw_val,
    elif feature_spec["dtype"] == "string":
        if raw_val is None:
            return "",
        else:
            return str(raw_val),
    else:
        # This case is used for unittests.
        # For example, explain_test.py uses int32 data.
        return raw_val,


LIMIT_PATTERN = re.compile("LIMIT\\s+([0-9]+)", flags=re.I)


def limit_select(select, n):
    """Make the select SQL statement with limited row number to query.

    Args:
        select (str): the select SQL statement.
        n (int): the limited row number to query.

    Returns:
        If n >= 0, return a new SQL statement which would query n row(s)
        at most. If n < 0, return the original SQL statement.
    """
    if n < 0:
        return select

    def replace_limit_num(matched_limit):
        num = int(matched_limit.group(1))
        return "LIMIT {}".format(min(num, n))

    if LIMIT_PATTERN.search(select) is None:
        idx = select.rfind(";")
        if idx < 0:
            idx = len(select)

        return select[0:idx] + " LIMIT {}".format(n) + select[idx:]
    else:
        return LIMIT_PATTERN.sub(repl=replace_limit_num, string=select)


def selected_columns_and_types(conn, select):
    """Get the columns and types returned by the select statement.

    Args:
        conn: the runtime.dbapi.Connection object.
        select (str): the select SQL statement.

    Returns:
        A tuple whose each element is (column_name, column_type).
    """
    select = select.strip().rstrip(";")
    select = limit_select(select, 1)
    rs = conn.query(select)
    column_info = rs.column_info()
    rs.close()
    return column_info


def selected_cols(conn, select):
    """Get selected column for given select

    Args:
        conn: a dbapi.Connection object
        select: a selection statement, for paiio driver
            this params is ignored

    Returns:
        Column names of the selection.
        When conn.driver is paiio, the columns are exactlly
        all columns in given connection table
    """
    if conn.driver == "paiio":
        name_and_type = conn.query().column_info()
    else:
        name_and_type = selected_columns_and_types(conn, select)
    return [item[0] for item in name_and_type]


def read_features_from_row(row,
                           select_cols,
                           feature_column_names,
                           feature_metas,
                           is_xgboost=False):
    features = []
    for name in feature_column_names:
        feature = read_feature(row[select_cols.index(name)],
                               feature_metas[name], name, is_xgboost)
        features.append(feature)
    return tuple(features)


def to_db_field_type(driver, dtype):
    """
    This method converts the dtype to a field type that the CREATE
    TABLE statement accepts.

    Args:
        driver (str): the DBMS driver type.
        dtype (str): the data type.

    Returns:
        A field type that the CREATE TABLE statement accepts.
    """
    if dtype in ["VARCHAR", "CHAR"]:
        if driver == "mysql":
            return dtype + "(255)"
        else:
            return "STRING"
    else:
        return dtype


def db_generator(conn, statement, label_meta=None):
    def reader():
        rs = conn.query(statement)

        reader.field_names = [item[0] for item in rs.column_info()]
        reader.field_types = [item[1] for item in rs.column_info()]

        if label_meta:
            try:
                label_idx = reader.field_names.index(
                    label_meta["feature_name"])
            except ValueError:
                # NOTE(typhoonzero): For clustering model, label_column_name
                # may not in reader.field_names when predicting.
                label_idx = None
        else:
            label_idx = None

        for row in rs:
            # NOTE: If there is no label clause in the extended SQL, the
            # default label value would be -1, the Model implementation
            # can determine use it or not.
            label = row[label_idx] if label_idx is not None else -1
            if label_meta and label_meta["delimiter"] != "":
                if label_meta["dtype"] == "float32":
                    label = np.fromstring(label,
                                          dtype=float,
                                          sep=label_meta["delimiter"])
                elif label_meta["dtype"] == "int64":
                    label = np.fromstring(label,
                                          dtype=int,
                                          sep=label_meta["delimiter"])
            if label_idx is None:
                yield list(row), None
            else:
                yield list(row), label
        rs.close()

    return reader


@contextlib.contextmanager
def buffered_db_writer(conn,
                       table_name,
                       table_schema,
                       buff_size=100,
                       slice_id=0):
    driver = conn.driver
    if driver == "maxcompute":
        w = db_writer.MaxComputeDBWriter(conn, table_name, table_schema,
                                         buff_size)
    elif driver == "mysql":
        w = db_writer.MySQLDBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "hive":
        w = db_writer.HiveDBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "paiio":
        w = db_writer.PAIMaxComputeDBWriter(table_name, table_schema,
                                            buff_size, slice_id)
    else:
        raise ValueError("unrecognized database driver: %s" % driver)

    try:
        yield w
    finally:
        w.close()


def get_table_schema(conn, table):
    """Get column name and type of given table

    Args:
        conn: a database connection, this function will leave it open
        table: table name or db.table

    Returns:
        Tuple of (field_name, field_type) tuples
    """
    return conn.get_table_schema(table)
