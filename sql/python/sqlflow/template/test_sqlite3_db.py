from __future__ import absolute_import
from __future__ import division
from __future__ import print_function

from unittest import TestCase

from sqlflow.template.db import SQLite3DataBase


class TestDB(TestCase):

    select_statement = "select * from test"

    def test_connect_and_query(self):
        db = SQLite3DataBase(":memory:", user=None, password=None, host=None, port=None)

        table_name = "test"
        table_schema = [("features", "text"), ("label", "int")]
        values = [('5,6,1,2', 1)] * 10

        db.create_table(table_name, table_schema)
        db.insert_values(table_name, table_schema, values)

        field_names, data = db.query_select(self.select_statement)

        expect_features = ['5,6,1,2'] * 10
        expect_labels = [1] * 10

        self.assertEqual(field_names, ['features', 'label'])
        self.assertEqual(expect_features, data[0])
        self.assertEqual(expect_labels, data[1])

