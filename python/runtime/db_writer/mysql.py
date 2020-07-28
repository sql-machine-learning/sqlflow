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


class MySQLDBWriter(BufferedDBWriter):
    """
    MySQLDBWriter is used to write the Python row data into
    the MySQL table.

    Args:
        conn: the database connection object.
        table_name (str): the MySQL table name.
        table_schema (list[str]): the column names of the MySQL table.
        buff_size (int): the buffer size to be flushed.
    """
    def __init__(self, conn, table_name, table_schema, buff_size):
        super().__init__(conn, table_name, table_schema, buff_size)
        self.statement = '''insert into {} ({}) values({})'''.format(
            self.table_name, ", ".join(self.table_schema),
            ", ".join(["%s"] * len(self.table_schema)))

    def flush(self):
        """
        Flush the row data into the MySQL table.

        Returns:
            None
        """
        cursor = self.conn.cursor()
        try:
            cursor.executemany(self.statement, self.rows)
            self.conn.commit()
        finally:
            cursor.close()
            self.rows = []
