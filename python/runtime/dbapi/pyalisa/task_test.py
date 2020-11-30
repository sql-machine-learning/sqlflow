# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# limitations under the License

import time
import unittest

from runtime import testing
from runtime.dbapi.pyalisa.config import Config
from runtime.dbapi.pyalisa.task import Task


@unittest.skipUnless(testing.get_driver() == "alisa", "Skip non-alisa test")
class TestTask(unittest.TestCase):
    def test_exec_sql_query(self):
        t = Task(Config.from_env())
        code = "SELECT \"Alice\" AS name, 28.3 AS age, 56000 AS salary;"
        time.sleep(2)
        res = t.exec_sql(code, resultful=True)
        # check schema, header
        cols = res["columns"]
        self.assertEqual(cols[0]["name"], "name")
        self.assertEqual(cols[0]["typ"], "string")
        self.assertEqual(cols[1]["name"], "age")
        self.assertEqual(cols[1]["typ"], "double")
        self.assertEqual(cols[2]["name"], "salary")
        self.assertEqual(cols[2]["typ"], "bigint")
        # check body
        self.assertEqual(len(res["body"]), 1)
        row = res["body"][0]
        self.assertEqual(len(row), 3)
        self.assertEqual(row[0], "Alice")
        self.assertEqual(row[1], "28.3")
        self.assertEqual(row[2], "56000")

    def test_exec_sql_hint_query(self):
        t = Task(Config.from_env())
        hint = "set odps.sql.select.output.format=\"HumanReadable\";"
        qry = "SELECT 1;"
        goodCode = hint + "\n" + qry
        time.sleep(2)
        t.exec_sql(goodCode)
        badCode = hint + qry
        with self.assertRaises(Exception):
            time.sleep(2)
            t.exec_sql(badCode)

    def test_exec_sql_exec(self):
        outtbl = "table_2"
        t = Task(Config.from_env())
        code = "CREATE TABLE IF NOT EXISTS {}(c1 STRING);".format(outtbl)
        time.sleep(2)
        t.exec_sql(code, resultful=False)
        code = "DESCRIBE {};".format(outtbl)
        time.sleep(2)
        res = t.exec_sql(code, resultful=True)
        self.assertTrue(len(res["body"]) > 0)

    def test_exec_pyodps(self):
        t = Task(Config.from_env())
        outtbl = "table_2"
        time.sleep(2)
        code = "DROP TABLE IF EXISTS {}".format(outtbl)
        t.exec_sql(code)

        code = """import argparse
if __name__ == "__main__":
    input_table_name = args['input_table']
    output_table_name = args['output_table']
    print(input_table_name)
    print(output_table_name)
    input_table = o.get_table(input_table_name)
    print(input_table.schema)
    output_table = o.create_table(output_table_name, input_table.schema)
        """
        args = "input_table=table_1 output_table={}".format(outtbl)
        time.sleep(2)
        res = t.exec_pyodps(code, args)
        print(res)


if __name__ == "__main__":
    unittest.main()
