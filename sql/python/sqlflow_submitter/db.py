# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
import numpy as np
import tensorflow as tf
import sqlflow_submitter.db_writer as db_writer

def connect(driver, database, user, password, host, port, auth=""):
    if driver == "mysql":
        # NOTE: use MySQLdb to avoid bugs like infinite reading:
        # https://bugs.mysql.com/bug.php?id=91971
        from MySQLdb import connect
        return connect(user=user,
                       passwd=password,
                       db=database,
                       host=host,
                       port=int(port))
    elif driver == "sqlite3":
        from sqlite3 import connect
        return connect(database)
    elif driver == "hive":
        from impala.dbapi import connect
        return connect(user=user,
                       password=password,
                       database=database,
                       host=host,
                       port=int(port),
                       auth_mechanism=auth)
    elif driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.connect(database, user, password, host)

    raise ValueError("unrecognized database driver: %s" % driver)

def db_generator(driver, conn, statement,
                 feature_column_names, label_column_name,
                 feature_specs, fetch_size=128):
    def reader():
        cursor = conn.cursor()
        cursor.execute(statement)
        if driver == "hive":
            field_names = None if cursor.description is None \
                else [i[0][i[0].find('.') + 1:] for i in cursor.description]
        else:
            field_names = None if cursor.description is None \
                else [i[0] for i in cursor.description]
        if label_column_name:
            label_idx = field_names.index(label_column_name)
        else:
            label_idx = None
        
        while True:
            rows = cursor.fetchmany(size = fetch_size)
            if not rows:
                break
            # NOTE: keep the connection while training or connection will lost if no activities appear.
            if driver == "mysql":
                conn.ping(True)
            for row in rows:
                label = row[label_idx] if label_idx is not None else None
                features = []
                for name in feature_column_names:
                    # FIXME(typhoonzero): Should use correct dtype here.
                    if feature_specs[name]["is_sparse"]:
                        indices = np.fromstring(row[field_names.index(name)], dtype=int, sep=feature_specs[name]["delimiter"])
                        indices = indices.reshape(indices.size, 1)
                        values = np.ones([indices.size], dtype=np.int32)
                        dense_shape = np.array(feature_specs[name]["shape"], dtype=np.int64)
                        cell = (indices, values, dense_shape)
                    else:
                        # Dense string vector
                        if feature_specs[name]["delimiter"] != "":
                            if feature_specs[name]["dtype"] == "float32":
                                cell = np.fromstring(row[field_names.index(name)], dtype=float, sep=feature_specs[name]["delimiter"])
                            elif feature_specs[name]["dtype"] == "int64":
                                cell = np.fromstring(row[field_names.index(name)], dtype=int, sep=feature_specs[name]["delimiter"])
                            else:
                                raise ValueError('unrecognize dtype {}'.format(feature_specs[name]["dtype"]))
                        else:
                            cell = row[field_names.index(name)]
                    features.append(cell)
                yield (tuple(features), [label])
            if len(rows) < fetch_size:
                break
        cursor.close()

    if driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.db_generator(conn, statement, feature_column_names,
                label_column_name, feature_specs, fetch_size)
    return reader


@contextlib.contextmanager
def buffered_db_writer(driver, conn, table_name, table_schema, buff_size=100):
    if driver == "maxcompute":
        w = db_writer.MaxComputeDBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "mysql":
        w = db_writer.MySQLDBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "sqlite3":
        w = db_writer.SQLite3DBWriter(conn, table_name, table_schema, buff_size)
    elif driver == "hive":
        w = db_writer.HiveDBWriter(conn, table_name, table_schema, buff_size)
    else:
        raise ValueError("unrecognized database driver: %s" % driver)

    try:
        yield w
    finally:
        w.close()
