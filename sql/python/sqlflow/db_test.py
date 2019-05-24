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

from sqlflow.db import connect, execute, insert_values


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
