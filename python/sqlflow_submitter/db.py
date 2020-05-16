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
import copy
import os
import re

import numpy as np
import sqlflow_submitter.db_writer as db_writer
import tensorflow as tf


def parseMySQLDSN(dsn):
    # [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
    user, passwd, host, port, database, config_str = re.findall(
        "^(\w*):(\w*)@tcp\(([.a-zA-Z0-9]*):([0-9]*)\)/(\w*)(\?.*)?$", dsn)[0]
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
        from sqlflow_submitter.maxcompute import MaxCompute
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
        return connect(user=user,
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
        return conn
    elif driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.connect(database, user, password, host)

    raise ValueError("unrecognized database driver: %s" % driver)


def read_feature(raw_val, feature_spec, feature_name):
    # FIXME(typhoonzero): Should use correct dtype here.
    if feature_spec["is_sparse"]:
        indices = np.fromstring(raw_val,
                                dtype=int,
                                sep=feature_spec["delimiter"])
        indices = indices.reshape(indices.size, 1)
        values = np.ones([indices.size], dtype=np.int32)
        dense_shape = np.array(feature_spec["shape"], dtype=np.int64)
        return (indices, values, dense_shape)
    elif feature_spec["delimiter"] != "":
        # Dense string vector
        if feature_spec["dtype"] == "float32":
            return np.fromstring(raw_val,
                                 dtype=float,
                                 sep=feature_spec["delimiter"])
        elif feature_spec["dtype"] == "int64":
            return np.fromstring(raw_val,
                                 dtype=int,
                                 sep=feature_spec["delimiter"])
        else:
            raise ValueError('unrecognize dtype {}'.format(
                feature_spec[feature_name]["dtype"]))
    else:
        return (raw_val, )


def selected_cols(driver, conn, select):
    select = select.strip().rstrip(";")
    limited = re.findall("LIMIT [0-9]*$", select.upper())
    if not limited:
        select += " LIMIT 1"

    if driver == "hive":
        cursor = conn.cursor(configuration=conn.session_cfg)
        cursor.execute(select)
        field_names = None if cursor.description is None \
            else [i[0][i[0].find('.') + 1:] for i in cursor.description]
    else:
        cursor = conn.cursor()
        cursor.execute(select)
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]
    cursor.close()
    return field_names


def read_features_from_row(row, select_cols, feature_column_names,
                           feature_specs):
    features = []
    for name in feature_column_names:
        feature = read_feature(row[select_cols.index(name)],
                               feature_specs[name], name)
        features.append(feature)
    return tuple(features)


def db_generator(driver,
                 conn,
                 statement,
                 feature_column_names,
                 label_spec,
                 feature_specs,
                 fetch_size=128):
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

        if label_spec:
            try:
                label_idx = field_names.index(label_spec["feature_name"])
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
                if label_spec and label_spec["delimiter"] != "":
                    if label_spec["dtype"] == "float32":
                        label = np.fromstring(label,
                                              dtype=float,
                                              sep=label_spec["delimiter"])
                    elif label_spec["dtype"] == "int64":
                        label = np.fromstring(label,
                                              dtype=int,
                                              sep=label_spec["delimiter"])
                if label_idx is None:
                    yield (list(row), None)
                else:
                    yield list(row), label
            if len(rows) < fetch_size:
                break
        cursor.close()

    if driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.db_generator(conn, statement, feature_column_names,
                                       label_spec, feature_specs, fetch_size)
    if driver == "hive":
        # trip the suffix ';' to avoid the ParseException in hive
        statement = statement.rstrip(';')
    return reader


def pai_maxcompute_db_generator(table,
                                feature_column_names,
                                label_column_name,
                                feature_specs,
                                fetch_size=128,
                                slice_id=0,
                                slice_count=1):
    def reader():
        selected_cols = copy.copy(feature_column_names)
        if label_column_name:
            selected_cols.append(label_column_name)
            try:
                label_idx = selected_cols.index(label_column_name)
            except ValueError:
                # NOTE(typhoonzero): For clustering model, label_column_name may not in field_names when predicting.
                label_idx = None
        else:
            label_idx = None

        import paiio
        reader = paiio.TableReader(table,
                                   selected_cols=",".join(selected_cols),
                                   slice_id=slice_id,
                                   slice_count=slice_count)
        while True:
            try:
                row = reader.read(num_records=1)[0]
                label = row[label_idx] if label_idx is not None else -1
                if label_column_name:
                    yield (list(row), None)
                else:
                    yield list(row), None
            except Exception as e:
                reader.close()
                break

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
