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
                       port=port)
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

    raise ValueError("unrecognized database driver: %s" % driver)


def execute(driver, conn, statement):
    cursor = conn.cursor()
    cursor.execute(statement)

    if driver == "hive":
        field_names = None if cursor.description is None \
            else [i[0][i[0].find('.') + 1:] for i in cursor.description]
    else:
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]

    try:
        rows = cursor.fetchall()
        field_columns = list(map(list, zip(*rows))) if len(rows) > 0 else None
    except:
        field_columns = None

    return field_names, field_columns


def db_generator(driver, conn, statement,
                 feature_column_names, label_column_name,
                 column_name_to_type, fetch_size=128):
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
            for row in rows:
                label = row[label_idx]
                features = dict()
                for name in feature_column_names:
                    if column_name_to_type[name] == "categorical_column_with_identity":
                        cell = np.fromstring(row[field_names.index(name)], dtype=int, sep=",")
                    else:
                        cell = row[field_names.index(name)]
                    features[name] = cell
                yield (features, [label])
            rows = cursor.fetchmany(fetch_size)
        cursor.close()
    return reader

def db_generator_predict(driver, conn, statement,
                         feature_column_names,
                         column_name_to_type, fetch_size=128):
    def reader():
        cursor = conn.cursor()
        cursor.execute(statement)
        if driver == "hive":
            field_names = None if cursor.description is None \
                else [i[0][i[0].find('.') + 1:] for i in cursor.description]
        else:
            field_names = None if cursor.description is None \
                else [i[0] for i in cursor.description]

        rows = cursor.fetchmany(fetch_size)
        while len(rows) > 0:
            for row in rows:
                features = dict()
                for name in feature_column_names:
                    if column_name_to_type[name] == "categorical_column_with_identity":
                        cell = np.fromstring(row[field_names.index(name)], dtype=int, sep=",")
                    else:
                        cell = row[field_names.index(name)]
                    features[name] = cell
                yield features
            rows = cursor.fetchmany(fetch_size)
        cursor.close()
    return reader

def insert_values(driver, conn, table_name, table_schema, values):
    if driver == "mysql":
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

    return cursor
