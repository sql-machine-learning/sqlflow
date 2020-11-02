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

import time
import unittest

from runtime import testing
from runtime.dbapi.pyalisa.client import AlisaTaksStatus, Client


@unittest.skipUnless(testing.get_driver() == "alisa", "Skip non-alisa test")
class TestClient(unittest.TestCase):
    def test_create_sql_task(self):
        ali = Client.from_env()
        code = "SELECT 2;"
        task_id, _ = ali.create_sql_task(code)
        self.assertIsNotNone(task_id)

        status = ali.get_status(task_id)
        self.assertIn(status, AlisaTaksStatus)

        # try get result
        for _ in range(10):
            time.sleep(1)
            status = ali.get_status(task_id)
            if ali.completed(status):
                count = ali.count_results(task_id)
                self.assertEqual(1, count)
                result = ali.get_results(task_id, 10)
                self.assertGreater(len(result), 0)
                break

    def test_create_pyodps_task(self):
        ali = Client.from_env()
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
        args = "input_table=table_1 output_table=table_2"
        task_id, _ = ali.create_pyodps_task(code, args)
        # to avoid touching the flow-control
        time.sleep(2)
        self.assertIsNotNone(task_id)


if __name__ == "__main__":
    unittest.main()
