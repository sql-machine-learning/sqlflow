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
# limitations under the License

import unittest
from unittest import TestCase

from runtime import testing
from runtime.dbapi.mysql_connection import MySQLConnection


@unittest.skipUnless(testing.get_driver() == "mysql", "Skip non-mysql test")
class TestMySQLConnection(TestCase):
    def test_connecion(self):
        try:
            conn = MySQLConnection(testing.get_datasource())
            conn.close()
        except:
            self.fail()

    def test_query(self):
        conn = MySQLConnection(testing.get_datasource())
        rs = conn.query("select * from notexist limit 1")
        self.assertFalse(rs.success())

        rs = conn.query("select * from train limit 1")
        self.assertTrue(rs.success())
        rows = [r for r in rs]
        self.assertEqual(1, len(rows))

        rs = conn.query("select * from train limit 20")
        self.assertTrue(rs.success())

        col_info = rs.column_info()
        self.assertEqual([('sepal_length', 'FLOAT'), ('sepal_width', 'FLOAT'),
                          ('petal_length', 'FLOAT'), ('petal_width', 'FLOAT'),
                          ('class', 'INT')], col_info)

        rows = [r for r in rs]
        self.assertTrue(20, len(rows))

    def test_exec(self):
        conn = MySQLConnection(testing.get_datasource())
        rs = conn.exec("create table test_exec(a int)")
        self.assertTrue(rs)
        rs = conn.exec("insert into test_exec values(1), (2)")
        self.assertTrue(rs)
        rs = conn.query("select * from test_exec")
        self.assertTrue(rs.success())
        rows = [r for r in rs]
        self.assertTrue(2, len(rows))
        rs = conn.exec("drop table test_exec")
        self.assertTrue(rs)
        rs = conn.exec("drop table not_exist")
        self.assertFalse(rs)


if __name__ == "__main__":
    unittest.main()
