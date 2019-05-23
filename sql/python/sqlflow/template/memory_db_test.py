from unittest import TestCase

from sqlflow.template.db import connect, execute, insert_values


class TestMemoryDB(TestCase):

    create_statement = "create table test (features text, label int)"
    select_statement = "select * from test"

    def test_sqlite3(self):
        driver = "sqlite3"

        conn = connect(driver, ":memory:", user=None, password=None, host=None, port=None)

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

