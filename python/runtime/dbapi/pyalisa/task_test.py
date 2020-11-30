# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# limitations under the License

import sys
import unittest

from runtime import testing
from runtime.dbapi.pyalisa.config import Config
from runtime.dbapi.pyalisa.task import Task


@unittest.skipUnless(testing.get_driver() == "alisa", "Skip non-alisa test")
class TestTask(unittest.TestCase):
    def test_exec_sql(self):
        t = Task(Config.from_env())
        code = "SELECT 2;"
        output = sys.stdout
        t.exec_sql(code, output, True)


if __name__ == "__main__":
    unittest.main()
