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
        self.tmp_f = tempfile.NamedTemporaryFile(dir="./")
        self.f = open(self.tmp_f.name, "w")

    def flush(self):
        for row in self.rows:
            line = "%s\n"  % '\001'.join([str(v) for v in row])
            self.f.write(line)
        self.rows = []

    def write_hive_table(self):
        hdfs_path = os.getenv("SQLFLOW_HIVE_LOCATION_ROOT_PATH", "/sqlflow")
        namenode_addr = os.getenv("SQLFLOW_HDFS_NAMENODE_ADDR", "127.0.0.1:8020")
        cmd_str = "hdfs dfs -copyFromLocal %s hdfs://%s%s/%s" % (self.tmp_f.name, namenode_addr, hdfs_path, self.table_name)
        subprocess.check_output(cmd_str.split())

    def close(self):
        try:
            if len(self.rows) > 0:
                self.flush()
            self.f.flush()
            self.write_hive_table()
        finally:
            self.f.close()
            self.tmp_f.close()
        