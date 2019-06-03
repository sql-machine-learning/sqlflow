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

from unittest import TestCase

import os

import tensorflow as tf
from sqlflow_submitter.db import connect, insert_values, db_generator, db_generator_predict


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


class TestDB(TestCase):

    create_statement = "create table test_db (features text, label int)"
    hive_create_statement = "create table test_db (features string, label int)"
    select_statement = "select * from test_db"
    drop_statement = "drop table if exists test_db"

    def test_sqlite3(self):
        driver = os.environ.get('SQLFLOW_TEST_DB') or "sqlite3"
        if driver == "sqlite3":
            conn = connect(driver, ":memory:", user=None, password=None, host=None, port=None)
            self._do_test(driver, conn)

    def test_mysql(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            user = os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root"
            password = os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root"
            host = "127.0.0.1"
            port = "3306"
            database = "iris"
            conn = connect(driver, database, user=user, password=password, host=host, port=port)
            self._do_test(driver, conn)

    def test_hive(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "hive":
            host = "127.0.0.1"
            port = "10000"
            conn = connect(driver, "iris", user="root", password="root", host=host, port=port)
            self._do_test(driver, conn)

    def _do_test(self, driver, conn):
        table_name = "test_db"
        table_schema = ["features", "label"]
        values = [('5,6,1,2', 1)] * 10

        execute(driver, conn, self.drop_statement)

        if driver == "hive":
            execute(driver, conn, self.hive_create_statement)
        else:
            execute(driver, conn, self.create_statement)

        insert_values(driver, conn, table_name, table_schema, values)

        field_names, data = execute(driver, conn, self.select_statement)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])


class TestGenerator(TestCase):
    create_statement = "create table test_table_float_fea (features float, label int)"
    drop_statement = "drop table if exists test_table_float_fea"
    insert_statement = "insert into test_table_float_fea (features,label) values(1.0, 0), (2.0, 1)"

    def test_generator(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            database = "iris"
            user = os.environ.get('SQLFLOW_TEST_DB_MYSQL_USER') or "root"
            password = os.environ.get('SQLFLOW_TEST_DB_MYSQL_PASSWD') or "root"
            conn = connect(driver, database, user=user, password=password, host="127.0.0.1", port="3306")
            # prepare test data
            execute(driver, conn, self.drop_statement)
            execute(driver, conn, self.create_statement)
            execute(driver, conn, self.insert_statement)

            column_name_to_type = {"features": tf.float32}
            gen = db_generator(driver, conn, "SELECT * FROM test_table_float_fea",
                               ["features"], "label", column_name_to_type)
            idx = 0
            for d in gen():
                if idx == 0:
                    self.assertEqual(d, ({"features": 1.0}, [0]))
                elif idx == 1:
                    self.assertEqual(d, ({"features": 2.0}, [1]))
                idx += 1
            self.assertEqual(idx, 2)

            gen_pred = db_generator_predict(driver, conn, "SELECT * FROM test_table_float_fea",
                                            ["features"], column_name_to_type)
            idx = 0
            for d in gen_pred():
                if idx == 0:
                    self.assertEqual(d, {"features": 1.0})
                elif idx == 1:
                    self.assertEqual(d, {"features": 2.0})
                idx += 1
            self.assertEqual(idx, 2)
