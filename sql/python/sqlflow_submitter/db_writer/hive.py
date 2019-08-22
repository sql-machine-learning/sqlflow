# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

import os

from .base import BufferedDBWriter
import tempfile
import subprocess

class HiveDBWriter(BufferedDBWriter):
    def __init__(self, conn, table_name, table_schema, buff_size=10000):
        super().__init__(conn, table_name, table_schema, buff_size)

    def flush(self):
        tmp_f = tempfile.NamedTemporaryFile()
        with open(tmp_f.name, "w") as f:
            for row in self.rows:
                line = "%s\n"  % '\001'.join([str(v) for v in row])
                f.write(line)

        hdfs_path = os.getenv("SQLFLOW_HIVE_HDFS_ROOT_PATH", "/sqlflow")
        cmd_str = "hdfs dfs -copyFromLocal %s hdfs://127.0.0.1:8020%s/%s" % (f.name, hdfs_path, self.table_name.replace(".", "_"))
        print(cmd_str)
        subprocess.run(cmd_str.split())
