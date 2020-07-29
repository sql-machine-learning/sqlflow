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

from IPython import get_ipython

ipython = get_ipython()


class TestSQLFlowMagic(unittest.TestCase):
    import random
    random.seed()
    tmp_db = "e2e_{}".format(random.randint(0, 1 << 32))
    # the standard SQL statement
    create_database_statement = "create database if not exists {}".format(
        tmp_db)
    create_statement = "create table {}.test_table_float_fea " \
                       "(features float, label int)".format(tmp_db)
    insert_statement = "insert into {}.test_table_float_fea (features,label)" \
                       " values(1.0, 0), (2.0, 1)".format(tmp_db)
    select_statement = "select * from {}.test_table_float_fea limit 1;".format(
        tmp_db)

    def setUp(self):
        ipython.run_cell_magic("sqlflow", "", self.create_database_statement)
        ipython.run_cell_magic("sqlflow", "", self.create_statement)
        ipython.run_cell_magic("sqlflow", "", self.insert_statement)

    def tearDown(self):
        pass

    def test_standard_sql(self):
        ret = ipython.run_cell_magic("sqlflow", "", self.select_statement)
        ret_list = [r for r in ret.get(0).rows()]
        self.assertEqual(len(ret_list), 1)
        self.assertEqual(ret_list[0], [1.0, 0])


if __name__ == "__main__":
    unittest.main()
