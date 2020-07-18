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

import unittest

import numpy as np
import runtime.db as db
import runtime.testing as testing
from runtime.verifier import fetch_samples, verify_column_name_and_type


def length(iterable):
    n = 0
    for _ in iterable:
        n += 1

    return n


class TestFetchSamples(unittest.TestCase):
    @unittest.skipUnless(testing.get_driver() in ["mysql", "hive"],
                         "skip non mysql/hive tests")
    def test_fetch_sample(self):
        conn = testing.get_singleton_db_connection()

        select = "SELECT * FROM iris.train"
        name_and_type = db.selected_columns_and_types(conn, select)
        expect_field_names = [item[0] for item in name_and_type]
        expect_field_types = [item[1] for item in name_and_type]
        column_num = len(name_and_type)

        gen = fetch_samples(conn, select, n=0)
        self.assertTrue(gen is None)

        gen = fetch_samples(conn, select, n=-1)
        row_num = length(gen())
        self.assertTrue(np.array_equal(gen.field_names, expect_field_names))
        self.assertTrue(np.array_equal(gen.field_types, expect_field_types))
        self.assertGreater(row_num, 25)

        gen = fetch_samples(conn, select, n=25)
        n = 0

        self.assertTrue(np.array_equal(gen.field_names, expect_field_names))
        self.assertTrue(np.array_equal(gen.field_types, expect_field_types))

        for rows in gen():
            self.assertEqual(len(rows), column_num)
            n += 1

        self.assertEqual(n, 25)

        gen = fetch_samples(conn, select, n=10)
        self.assertTrue(np.array_equal(gen.field_names, expect_field_names))
        self.assertTrue(np.array_equal(gen.field_types, expect_field_types))
        self.assertEqual(length(gen()), 10)

        gen = fetch_samples(conn, "%s LIMIT 1" % select, n=1000)
        self.assertTrue(np.array_equal(gen.field_names, expect_field_names))
        self.assertTrue(np.array_equal(gen.field_types, expect_field_types))
        self.assertEqual(length(gen()), 1)

        gen = fetch_samples(conn, select, n=row_num * 2)
        self.assertTrue(np.array_equal(gen.field_names, expect_field_names))
        self.assertTrue(np.array_equal(gen.field_types, expect_field_types))
        self.assertEqual(length(gen()), row_num)


class TestFetchVerifyColumnNameAndType(unittest.TestCase):
    def generate_select(self, table, columns):
        return "SELECT %s FROM %s" % (",".join(columns), table)

    @unittest.skipUnless(testing.get_driver() in ["mysql", "hive"],
                         "skip non mysql/hive tests")
    def test_verify_column_name_and_type(self):
        conn = testing.get_singleton_db_connection()

        train_table = "iris.train"
        test_table = "iris.test"

        train_select = [
            "petal_length", "petal_width", "sepal_length", "sepal_width",
            "class"
        ]
        test_select = train_select
        verify_column_name_and_type(
            conn, self.generate_select(train_table, train_select),
            self.generate_select(test_table, test_select), "class")

        test_select = [
            "petal_length", "petal_width", "sepal_length", "sepal_width"
        ]
        verify_column_name_and_type(
            conn, self.generate_select(train_table, train_select),
            self.generate_select(test_table, test_select), "class")

        test_select = ["petal_length", "petal_width", "sepal_length"]
        with self.assertRaises(ValueError):
            verify_column_name_and_type(
                conn, self.generate_select(train_table, train_select),
                self.generate_select(test_table, test_select), "class")

        cursor = conn.cursor()
        name_and_type = dict(db.get_table_schema(conn, test_table))
        new_table_name = "iris.verifier_test_table"

        name_and_type["petal_length"] = "VARCHAR(255)"  # change the data type
        create_column_str = ",".join(
            ["%s %s" % (n, t) for n, t in name_and_type.items()])

        drop_sql = "DROP TABLE IF EXISTS %s" % new_table_name
        create_sql = "CREATE TABLE %s(%s)" % (new_table_name,
                                              create_column_str)
        cursor.execute(drop_sql)
        cursor.execute(create_sql)
        with self.assertRaises(ValueError):
            test_select = train_select
            verify_column_name_and_type(
                conn, self.generate_select(train_table, train_select),
                self.generate_select(new_table_name, test_select), "class")
        cursor.execute(drop_sql)
        cursor.close()


if __name__ == "__main__":
    unittest.main()
