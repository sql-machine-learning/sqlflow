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

class MySQLDBWriter(DBWriter):
    def __init__(self, conn, table_name, table_schema, buff_size):
        return super().__init__(conn, table_name, table_schema, buff_size)

    def flush(self):
        statement = '''insert into table {} ({}) values({})'''.format(
            table_name,
            ", ".join(table_schema),
            ", ".join(["%s"] * len(table_schema))
        )
        cursor = self.conn.cursor()
        cursor.executemany(statement, self.rows)
        self.conn.commit()
        cursor.close()
        self.rows = []

    def write(self, value):
        self.rows.append(value)
        if len(self.rows) > self.buff_size:
            self.flush()

    def close(self):
        if len(self.rows) > 0:
            self.flush()
