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
from odps import ODPS, tunnel


def parseMySQLDSN(dsn):
    # [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
    user, passwd, host, port, database, config_str = re.findall(
        "^(\w*):(\w*)@tcp\(([.a-zA-Z0-9\-]*):([0-9]*)\)/(\w*)(\?.*)?$", dsn)[0]
    config = {}
    if len(config_str) > 1:
        for c in config_str[1:].split("&"):
            k, v = c.split("=")
            config[k] = v
    return user, passwd, host, port, database, config


def parseHiveDSN(dsn):
    # usr:pswd@hiveserver:10000/mydb?auth=PLAIN&session.mapreduce_job_quenename=mr
    user_passwd, address_database, config_str = re.findall(
        "^(.*)@([.a-zA-Z0-9/:_]*)(\?.*)?", dsn)[0]
    user, passwd = user_passwd.split(":")
    if len(address_database.split("/")) > 1:
        address, database = address_database.split("/")
    else:
        address, database = address_database, None
    if len(address.split(":")) > 1:
        host, port = address.split(":")
    else:
        host, port = address, None
    config = {}
    if len(config_str) > 1:
        for c in config_str[1:].split("&"):
            k, v = c.split("=")
            config[k] = v
    auth = config["auth"] if "auth" in config else ""
    session = {}
    for k, v in config.items():
        if k.startswith("session."):
            session[k[len("session."):]] = v
    return user, passwd, host, port, database, auth, session


def parseMaxComputeDSN(dsn):
    # access_id:access_key@service.com/api?curr_project=test_ci&scheme=http
    user_passwd, address, config_str = re.findall(
        "^(.*)@([-.a-zA-Z0-9/]*)(\?.*)?", dsn)[0]
    user, passwd = user_passwd.split(":")
    config = {}
    if len(config_str) > 1:
        for c in config_str[1:].split("&"):
            k, v = c.split("=")
            config[k] = v
    if "scheme" in config:
        address = config["scheme"] + "://" + address
    return user, passwd, address, config["curr_project"]


def connect_with_data_source(driver_dsn):
    driver, dsn = driver_dsn.split("://")
    if driver == "mysql":
        # NOTE: use MySQLdb to avoid bugs like infinite reading:
        # https://bugs.mysql.com/bug.php?id=91971
        from MySQLdb import connect
        user, passwd, host, port, database, config = parseMySQLDSN(dsn)
        conn = connect(user=user,
                       passwd=passwd,
                       db=database,
                       host=host,
                       port=int(port))
    elif driver == "hive":
        from impala.dbapi import connect
        user, passwd, host, port, database, auth, session_cfg = parseHiveDSN(
            dsn)
        conn = connect(user=user,
                       password=passwd,
                       database=database,
                       host=host,
                       port=int(port),
                       auth_mechanism=auth)
        conn.session_cfg = session_cfg
        conn.default_db = database
    elif driver == "maxcompute":
        from runtime.maxcompute import MaxCompute
        user, passwd, address, database = parseMaxComputeDSN(dsn)
        conn = MaxCompute.connect(database, user, passwd, address)
    else:
        raise ValueError(
            "connect_with_data_source doesn't support driver type {}".format(
                driver))

    conn.driver = driver
    return conn


def connect(driver,
            database,
            user,
            password,
            host,
            port,
            session_cfg={},
            auth=""):
    if driver == "mysql":
        # NOTE: use MySQLdb to avoid bugs like infinite reading:
        # https://bugs.mysql.com/bug.php?id=91971
        from MySQLdb import connect
        conn = connect(user=user,
                       passwd=password,
                       db=database,
                       host=host,
                       port=int(port))
    elif driver == "hive":
        from impala.dbapi import connect
        conn = connect(user=user,
                       password=password,
                       database=database,
                       host=host,
                       port=int(port),
                       auth_mechanism=auth)
        conn.default_db = database
        conn.session_cfg = session_cfg
    elif driver == "maxcompute":
        from runtime.maxcompute import MaxCompute
        conn = MaxCompute.connect(database, user, password, host)
    else:
        raise ValueError("unrecognized database driver: %s" % driver)

    conn.driver = driver
    return conn


def read_feature(raw_val, feature_spec, feature_name):
    # FIXME(typhoonzero): Should use correct dtype here.
    if feature_spec["is_sparse"]:
        if feature_spec["format"] == "kv":
            items = raw_val.split()
            items = [item.split(':', 2) for item in items]
            indices = np.array([int(item[0]) for item in items],
                               dtype=np.int64)
            values = np.array([float(item[1]) for item in items],
                              dtype=np.float32)
        else:
            indices = np.fromstring(raw_val,
                                    dtype=int,
                                    sep=feature_spec["delimiter"])
            indices = indices.reshape(indices.size, 1)
            values = np.ones([indices.size], dtype=np.int64)

        dense_shape = np.array(feature_spec["shape"], dtype=np.int64)
        return indices, values, dense_shape
    elif feature_spec["delimiter"] != "":
        # Dense string vector
        if feature_spec["dtype"] == "float32":
            return np.fromstring(raw_val,
                                 dtype=np.float32,
                                 sep=feature_spec["delimiter"])
        elif feature_spec["dtype"] == "int64":
            return np.fromstring(raw_val,
                                 dtype=np.int64,
                                 sep=feature_spec["delimiter"])
        else:
            raise ValueError('unrecognize dtype {}'.format(
                feature_spec[feature_name]["dtype"]))
    elif feature_spec["dtype"] == "float32":
        return float(raw_val),
    elif feature_spec["dtype"] == "int64":
        int_raw_val = long(raw_val) if six.PY2 else int(raw_val)
        return int_raw_val,
    elif feature_spec["dtype"] == "string":
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
        If n >= 0, return a new SQL statement which would query n row(s) at most.
        If n < 0, return the original SQL statement.
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


try:
    import MySQLdb.constants.FIELD_TYPE as MYSQL_FIELD_TYPE
    # Refer to http://mysql-python.sourceforge.net/MySQLdb-1.2.2/public/MySQLdb.constants.FIELD_TYPE-module.html
    MYSQL_DATA_TYPE_DICT = {
        MYSQL_FIELD_TYPE.TINY: "TINYINT",  # 1
        MYSQL_FIELD_TYPE.LONG: "INT",  # 3
        MYSQL_FIELD_TYPE.FLOAT: "FLOAT",  # 4
        MYSQL_FIELD_TYPE.DOUBLE: "DOUBLE",  # 5
        MYSQL_FIELD_TYPE.LONGLONG: "BIGINT",  # 8
        MYSQL_FIELD_TYPE.NEWDECIMAL: "DECIMAL",  # 246
        MYSQL_FIELD_TYPE.BLOB: "TEXT",  # 252
        MYSQL_FIELD_TYPE.VAR_STRING: "VARCHAR",  # 253
        MYSQL_FIELD_TYPE.STRING: "CHAR",  # 254
    }
except:
    MYSQL_DATA_TYPE_DICT = {}


def selected_columns_and_types(conn, select):
    """Get the columns and types returned by the select statement.

    Args:
        conn: the connection object.
        select (str): the select SQL statement.

    Returns:
        A tuple whose each element is (column_name, column_type).
    """
    select = select.strip().rstrip(";")
    select = limit_select(select, 1)

    driver = conn.driver
    if driver == "mysql":
        cursor = conn.cursor()
        cursor.execute(select)
        try:
            name_and_type = []
            for desc in cursor.description:
                # NOTE: MySQL returns an integer number instead of a string
                # to represent the data type.
                typ = MYSQL_DATA_TYPE_DICT.get(desc[1])
                if typ is None:
                    raise ValueError(
                        "unsupported data type of column {}".format(desc[0]))
                name_and_type.append((desc[0], typ))
            return name_and_type
        finally:
            cursor.close()

    if driver == "hive":
        cursor = conn.cursor(configuration=conn.session_cfg)
        cursor.execute(select)
        name_and_type = []
        for desc in cursor.description:
            name = desc[0].split('.')[-1]
            name_and_type.append((name, desc[1]))
        cursor.close()
        return name_and_type

    if driver == "maxcompute":
        from runtime.maxcompute import MaxCompute
        return MaxCompute.selected_columns_and_types(conn, select)

    raise NotImplementedError("unsupported driver {}".format(driver))


def selected_cols(conn, select):
    name_and_type = selected_columns_and_types(conn, select)
    return [item[0] for item in name_and_type]


def pai_selected_cols(table):
    import paiio
    reader = paiio.TableReader(table)
    schema = reader.get_schema()
    selected_cols = [i['colname'] for i in schema]
    reader.close()
    return selected_cols


def get_pai_table_row_num(table):
    import paiio
    reader = paiio.TableReader(table)
    row_num = reader.get_row_count()
    reader.close()
    return row_num


def read_features_from_row(row, select_cols, feature_column_names,
                           feature_metas):
    features = []
    for name in feature_column_names:
        feature = read_feature(row[select_cols.index(name)],
                               feature_metas[name], name)
        features.append(feature)
    return tuple(features)


def db_generator(conn, statement, label_meta=None, fetch_size=128):
    driver = conn.driver

    def reader():
        if driver == "hive":
            cursor = conn.cursor(configuration=conn.session_cfg)
        else:
            cursor = conn.cursor()
        cursor.execute(statement)
        if driver == "hive":
            field_names = None if cursor.description is None \
                else [i[0][i[0].find('.') + 1:] for i in cursor.description]
        else:
            field_names = None if cursor.description is None \
                else [i[0] for i in cursor.description]

        if label_meta:
            try:
                label_idx = field_names.index(label_meta["feature_name"])
            except ValueError:
                # NOTE(typhoonzero): For clustering model, label_column_name may not in field_names when predicting.
                label_idx = None
        else:
            label_idx = None

        while True:
            rows = cursor.fetchmany(size=fetch_size)
            if not rows:
                break
            # NOTE: keep the connection while training or connection will lost if no activities appear.
            if driver == "mysql":
                conn.ping(True)
            for row in rows:
                # NOTE: If there is no label clause in the extended SQL, the default label value would
                # be -1, the Model implementation can determine use it or not.
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
            if len(rows) < fetch_size:
                break
        cursor.close()

    if driver == "maxcompute":
        from runtime.maxcompute import MaxCompute
        return MaxCompute.db_generator(conn, statement, label_meta, fetch_size)
    if driver == "hive":
        # trip the suffix ';' to avoid the ParseException in hive
        statement = statement.rstrip(';')
    return reader


def pai_maxcompute_db_generator(table,
                                label_column_name=None,
                                slice_id=0,
                                slice_count=1):
    def reader():
        import paiio
        pai_reader = paiio.TableReader(table,
                                       slice_id=slice_id,
                                       slice_count=slice_count)

        selected_cols = [item['colname'] for item in pai_reader.get_schema()]
        label_index = selected_cols.index(
            label_column_name) if label_column_name else None

        while True:
            try:
                row = pai_reader.read(num_records=1)[0]
            except:
                pai_reader.close()
                break

            if label_index is not None:
                yield list(row), row[label_index]
            else:
                yield list(row), None

    return reader


@contextlib.contextmanager
def buffered_db_writer(driver,
                       conn,
                       table_name,
                       table_schema,
                       buff_size=100,
                       hdfs_namenode_addr="",
                       hive_location="",
                       hdfs_user="",
                       hdfs_pass=""):
    if driver == "maxcompute":
        w = db_writer.MaxComputeDBWriter(conn, table_name, table_schema,
                                         buff_size)
    elif driver == "mysql":
        w = db_writer.MySQLDBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "hive":
        w = db_writer.HiveDBWriter(conn,
                                   table_name,
                                   table_schema,
                                   buff_size,
                                   hdfs_namenode_addr=hdfs_namenode_addr,
                                   hive_location=hive_location,
                                   hdfs_user=hdfs_user,
                                   hdfs_pass=hdfs_pass)
    elif driver == "pai_maxcompute":
        w = db_writer.PAIMaxComputeDBWriter(table_name, table_schema,
                                            buff_size)
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
    if conn.driver == "maxcompute":
        schema = conn.get_table(table).schema
        names_and_types = [(c.name, str(c.type).upper())
                           for c in schema.columns]
        return names_and_types
    else:
        statement = "describe %s" % table
        cursor = conn.cursor()
        cursor.execute(statement)
        fields = []
        for field in cursor:
            # add field name and type
            fields.append((field[0], field[1].upper()))
        cursor.close()
    return fields


def execute(conn, sql_stmt):
    """Execute the given sql statement and return True on success

    Args:
        conn: a database connection, this function will leave it open
        sql_stmt: the sql statement to execute
    
    Returns:
        True on success and False on failure
    """
    if conn.driver == "maxcompute":
        inst = conn.execute_sql(sql_stmt)
        return inst.is_successful
    else:
        try:
            cur = conn.cursor()
            cur.execute(sql_stmt)
            conn.commit()
            cur.close()
            return True
        except:
            return False
