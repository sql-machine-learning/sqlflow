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

from runtime.db_writer.base import BufferedDBWriter


class MaxComputeDBWriter(BufferedDBWriter):
    """
    MaxComputeDBWriter is used to write the Python row data into
    the MaxCompute table.

    Args:
        conn: the database connection object.
        table_name (str): the MaxCompute table name.
        table_schema (list[str]): the column names of the MaxCompute table.
        buff_size (int): the buffer size to be flushed.
    """
    def __init__(self, conn, table_name, table_schema, buff_size):
        super(MaxComputeDBWriter, self).__init__(conn, table_name,
                                                 table_schema, buff_size)

        # NOTE: import odps here instead of in the front of this file,
        # so that we do not need the odps package installed in the Docker
        # image if we do not use MaxComputeDBWriter.
        from odps import tunnel
        self.compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB

    def flush(self):
        """
        Flush the row data into the MaxCompute table.

        Returns:
            None
        """
        self.conn.write_table(self.table_name,
                              self.rows,
                              compress_option=self.compress)
        self.rows = []
