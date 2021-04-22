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

from .base import BufferedDBWriter
import json

class ClickhouseDBWriter(BufferedDBWriter):
    """
    ClickhouseDBWriter is used to write the Python row data into
    the Clickhouse table.

    Args:
        conn: the database connection object.
        table_name (str): the Clickhouse table name.
        table_schema (list[str]): the column names of the Clickhouse table.
        buff_size (int): the buffer size to be flushed.
    """
    def __init__(self, conn, table_name, table_schema, buff_size):
        super().__init__(conn, table_name, table_schema, buff_size)
        self.statement = '''insert into {} ({}) values '''.format(
            self.table_name, ", ".join(self.table_schema))

    def flush(self):
        """
        Flush the row data into the Clickhouse table.

        Returns:
            None
        """
        cursor = self.conn.cursor()
        try:
            cursor.set_types_check(True)
            cursor.executemany(self.statement, self.rows)
            self.conn.commit()
        finally:
            cursor.close()
            self.rows = []
