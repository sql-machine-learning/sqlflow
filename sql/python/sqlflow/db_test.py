from unittest import TestCase

import os

from sqlflow.db import connect, execute, insert_values


class TestDB(TestCase):

    create_statement = "create table test (features text, label int)"
    select_statement = "select * from test"

    def test_sqlite3(self):
        driver = os.environ.get('SQLFLOW_TEST_DB') or "sqlite3"
        if driver == "sqlite3":
            conn = connect(driver, ":memory:", user=None, password=None, host=None, port=None)
            self._do_test(driver, conn)

    def test_mysql(self):
        driver = os.environ.get('SQLFLOW_TEST_DB')
        if driver == "mysql":
            user = os.environ['SQLFLOW_TEST_DB_MYSQL_USER'] or "root"
            password = os.environ['SQLFLOW_TEST_DB_MYSQL_PASSWD'] or "root"
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
        table_name = "test"
        table_schema = [("features", "text"), ("label", "int")]
        values = [('5,6,1,2', 1)] * 10

        execute(driver, conn, self.create_statement)
        insert_values(driver, conn, table_name, table_schema, values)

        field_names, data = execute(driver, conn, self.select_statement)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])
