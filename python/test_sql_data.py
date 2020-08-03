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

import runtime.testing as testing
import sql_data
from runtime.db import parseMySQLDSN


class TestSQLData(unittest.TestCase):
    def __init__(self, *args, **kwargs):
        super(TestSQLData, self).__init__(*args, **kwargs)
        dsn = testing.get_mysql_dsn()
        user, passwd, host, port, _, _ = parseMySQLDSN(dsn)
        self.db = sql_data.connect(user, passwd, host, int(port))
        self.assertIsNotNone(self.db)

    def test_load(self):
        f, label = sql_data.load(self.db, 'SELECT * FROM iris.train LIMIT 3',
                                 'class', None)
        self.assertEqual(4, len(f.keys()))  # 4 features
        self.assertEqual(3, len(label))  # label column length

    def test_load_with_filter(self):
        fs = ['sepal_length', 'petal_width']
        f, label = sql_data.load(self.db, 'SELECT * FROM iris.train LIMIT 3',
                                 'class', fs)
        self.assertEqual(len(fs), len(f))
        self.assertEqual(3, len(label))  # label column length

    def test_feature_columns(self):
        f, label = sql_data.load(self.db, 'SELECT * FROM iris.train LIMIT 3',
                                 'class', None)
        c = sql_data.feature_columns(f)
        self.assertEqual(4, len(c))  # 4 features


if __name__ == '__main__':
    unittest.main()
