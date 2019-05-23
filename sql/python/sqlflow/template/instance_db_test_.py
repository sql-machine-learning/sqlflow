import os

from unittest import TestCase

from sqlflow.template.db import connect, execute, insert_values


class TestInstanceDB(TestCase):

    driver = os.environ['SQLFLOW_TEST_DB']

    create_statement = "create table test (features text, label int)"
    select_statement = "select * from test"

    def test_db(self):
        if self.driver == "mysql":
            user = os.environ['SQLFLOW_TEST_DB_MYSQL_USER'] or "root"
            password = os.environ['SQLFLOW_TEST_DB_MYSQL_PASSWD'] or "root"
            host = "127.0.0.1"
            port = "3306"
            database = "sqlflow_models"
            conn = connect(self.driver, database, user=user, password=password, host=host, port=port)
        elif self.driver == "hive":
            host = "127.0.0.1"
            port = "10000"
            conn = connect(self.driver, "iris", user="root", password="root", host=host, port=port)
        else:
            raise ValueError("unrecognized database driver: %s" % self.driver)

        return self._do_test(conn)

    def _do_test(self, conn):
        table_name = "test"
        table_schema = [("features", "text"), ("label", "int")]
        values = [('5,6,1,2', 1)] * 10

        execute(self.driver, conn, self.create_statement)
        insert_values(self.driver, conn, table_name, table_schema, values)

        field_names, data = execute(self.driver, conn, self.select_statement)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])

