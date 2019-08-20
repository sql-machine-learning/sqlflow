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

from .db_writer import DBWriter
from odps import ODPS, tunnel

class MaxComputeDBWriter(DBWriter):
    def __init__(self, conn, table_name, table_schema, buff_size):
        return super().__init__(conn, table_name, table_schema, buff_size)

    def flush(self):
        compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
        self.conn.write_table(self.table, self.rows, compress_option=compress)
        self.rows = []

    def write(self, value):
        self.rows.append(value)
        if len(self.rows) > self.buff_size:
            self.flush()

    def close(self):
        if len(self.rows) > 0:
            self.flush()
