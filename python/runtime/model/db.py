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

import base64

from runtime.db import buffered_db_writer, connect_with_data_source
from runtime.diagnostics import SQLFlowDiagnostic


def _create_table(conn, table):
    if conn.driver == "mysql":
        stmt = "CREATE TABLE IF NOT EXISTS {0} (id INT, block TEXT,\
        PRIMARY KEY (id))".format(table)
    elif conn.driver == "hive":
        stmt = 'CREATE TABLE IF NOT EXISTS {0} (id INT, block STRING) ROW\
            FORMAT DELIMITED FIELDS TERMINATED BY "\\001" \
                STORED AS TEXTFILE'.format(table)
    elif conn.driver == "maxcompute":
        stmt = "CREATE TABLE IF NOT EXISTS {0} (id INT,\
            block STRING)".format(table)
    else:
        raise SQLFlowDiagnostic("unsupported driver {0} on creating\
            table.".format(conn.driver))

    cursor = conn.cursor()
    cursor.execute(stmt)


def _drop_table_if_exists(conn, table):
    sql = "DROP TABLE IF EXISTS {0}".format(table)
    cursor = conn.cursor()
    cursor.execute(sql)


def write_with_generator(datasource, table, gen):
    """Write data into a table, the written data
    comes from the input generator.
    """
    conn = connect_with_data_source(datasource)
    _drop_table_if_exists(conn, table)
    _create_table(conn, table)
    idx = 0

    with buffered_db_writer(conn.driver, conn, table, ["id", "block"]) as w:
        for d in gen():
            block = base64.b64encode(d)
            row = [idx, block]
            w.write(row)
            idx += 1

    conn.close()


def read_with_generator(datasource, table):
    """Read data from a table, this function returns
    a generator to yield the data.
    """
    conn = connect_with_data_source(datasource)
    sql = "SELECT id, block FROM {0} ORDER BY id".format(table)
    cursor = conn.cursor()
    cursor.execute(sql)
    fetch_size = 100

    def reader():
        while True:
            rows = cursor.fetchmany(size=fetch_size)
            if not rows:
                break
            for r in rows:
                yield base64.b64decode(r[1])
        conn.close()

    return reader
