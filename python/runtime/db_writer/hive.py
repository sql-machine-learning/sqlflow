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

import os
import subprocess
import tempfile

from .base import BufferedDBWriter

CSV_DELIMITER = '\001'


class HiveDBWriter(BufferedDBWriter):
    def __init__(self, conn, table_name, table_schema, buff_size=10000):
        super().__init__(conn, table_name, table_schema, buff_size)
        self.tmp_f = tempfile.NamedTemporaryFile(dir="./")
        self.f = open(self.tmp_f.name, "w")
        self.schema_idx = self._indexing_table_schema(table_schema)
        self.hdfs_namenode_addr = conn.param("hdfs_namenode_addr")
        self.hive_location = conn.param("hive_location")
        self.hdfs_user = conn.uripts.username
        self.hdfs_pass = conn.uripts.password

    def _indexing_table_schema(self, table_schema):
        column_list = self.conn.get_table_schema(self.table_name)

        schema_idx = []
        idx_map = {}
        # column list: [(col1, type, desc), (col2, type, desc)...]
        for i, e in enumerate(column_list):
            idx_map[e[0]] = i

        for s in table_schema:
            if s not in idx_map:
                raise ValueError("column: %s should be in table columns:%s" %
                                 (s, idx_map))
            schema_idx.append(idx_map[s])

        return schema_idx

    def _ordered_row_data(self, row):
        # Use NULL as the default value for hive columns
        row_data = ["NULL" for _ in range(len(self.table_schema))]
        for idx, element in enumerate(row):
            row_data[self.schema_idx[idx]] = str(element)
        return CSV_DELIMITER.join(row_data)

    def flush(self):
        for row in self.rows:
            data = self._ordered_row_data(row)
            self.f.write(data + '\n')
        self.f.flush()
        self.rows = []

    def write_hive_table(self):
        if self.hive_location == "":
            hdfs_path = os.getenv("SQLFLOW_HIVE_LOCATION_ROOT_PATH",
                                  "/sqlflow")
        else:
            hdfs_path = self.hive_location

        # upload CSV to HDFS
        hdfs_envs = os.environ.copy()
        if self.hdfs_user != "":
            hdfs_envs.update({"HADOOP_USER_NAME": self.hdfs_user})
        if self.hdfs_pass != "":
            hdfs_envs.update({"HADOOP_USER_PASSWORD": self.hdfs_pass})
        # if namenode_addr is not set, use local hdfs command's configuration.
        if self.hdfs_namenode_addr != "":
            cmd_namenode_str = "-fs hdfs://%s" % (self.hdfs_namenode_addr)
        else:
            cmd_namenode_str = ""
        cmd_str = "hdfs dfs %s -mkdir -p %s/%s/" % (cmd_namenode_str,
                                                    hdfs_path, self.table_name)
        subprocess.check_output(cmd_str.split(), env=hdfs_envs)
        cmd_str = "hdfs dfs %s -copyFromLocal %s %s/%s/" % (
            cmd_namenode_str, self.tmp_f.name, hdfs_path, self.table_name)
        subprocess.check_output(cmd_str.split(), env=hdfs_envs)
        # load CSV into Hive
        load_sql = "LOAD DATA INPATH '%s/%s/' OVERWRITE INTO TABLE %s" % (
            hdfs_path, self.table_name, self.table_name)
        self.conn.execute(load_sql)

        # remove the temporary dir on hdfs
        cmd_str = "hdfs dfs %s -rm -r -f %s/%s/" % (cmd_namenode_str,
                                                    hdfs_path, self.table_name)
        subprocess.check_output(cmd_str.split(), env=hdfs_envs)

    def close(self):
        try:
            if len(self.rows) > 0:
                self.flush()
            self.f.flush()
            self.write_hive_table()
        finally:
            self.f.close()
            self.tmp_f.close()
