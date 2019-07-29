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

import numpy as np
import tensorflow as tf

def connect(driver, database, user, password, host, port):
    if driver == "mysql":
        from mysql.connector import connect
        return connect(user=user,
                       passwd=password,
                       database=database,
                       host=host,
                       port=port,
                       connection_timeout=3600)
    elif driver == "sqlite3":
        from sqlite3 import connect
        return connect(database)
    elif driver == "hive":
        from impala.dbapi import connect
        return connect(user=user,
                       password=password,
                       database=database,
                       host=host,
                       port=int(port))
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
        label_idx = field_names.index(label_column_name)

        rows = cursor.fetchmany(fetch_size)
        while len(rows) > 0:
            # NOTE: keep the connection while training or connection will lost if no activities appear.
            # FIXME(Yancey1989): tempory comment this reconnect, because it caused to loss the cursor failed,
            # github issue: 
            #if driver == "mysql" and not conn.is_connected():
            #    conn.ping(True)
            for row in rows:
                label = row[label_idx]
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
            rows = cursor.fetchmany(fetch_size)
        cursor.close()

    if driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.db_generator(conn, statement, feature_column_names,
                label_column_name, feature_specs, fetch_size)
    return reader

def insert_values(driver, conn, table_name, table_schema, values):
    if driver == "maxcompute":
        from sqlflow_submitter.maxcompute import MaxCompute
        return MaxCompute.insert_values(conn, table_name, values)
    elif driver == "mysql":
        statement = '''insert into {} ({}) values({})'''.format(
            table_name,
            ", ".join(table_schema),
            ", ".join(["%s"] * len(table_schema))
        )
    elif driver == "sqlite3":
        statement = '''insert into {} ({}) values({})'''.format(
            table_name,
            ", ".join(table_schema),
            ", ".join(["?"] * len(table_schema))
        )
    elif driver == "hive":
        statement = '''insert into table {} ({}) values({})'''.format(
            table_name,
            ", ".join(table_schema),
            ", ".join(["%s"] * len(table_schema))
        )
    else:
        raise ValueError("unrecognized database driver: %s" % driver)

    cursor = conn.cursor()
    cursor.executemany(statement, values)
    conn.commit()
    cursor.close()
