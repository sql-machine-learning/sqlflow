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
import json

import six
from runtime.db import buffered_db_writer, connect_with_data_source
from runtime.diagnostics import SQLFlowDiagnostic
from runtime.feature.column import (JSONDecoderWithFeatureColumn,
                                    JSONEncoderWithFeatureColumn)


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

    conn.execute(stmt)


def _drop_table_if_exists(conn, table):
    sql = "DROP TABLE IF EXISTS {0}".format(table)
    conn.execute(sql)


class SQLFSWriter(object):
    def __init__(self, conn, table):
        self.context_manager = buffered_db_writer(conn, table, ["id", "block"])
        self.writer = self.context_manager.__enter__()
        self.row_idx = 0

    def write(self, content):
        block = base64.b64encode(content)
        if six.PY3 and isinstance(block, bytes):
            block = block.decode("utf-8")
        self.writer.write([self.row_idx, block])
        self.row_idx += 1

    def close(self):
        self.writer.close()

    def __enter__(self, *args, **kwargs):
        return self

    def __exit__(self, *args, **kwargs):
        self.context_manager.__exit__(*args, **kwargs)


def _build_ordered_reader(reader):
    block_dict = dict()
    cur_id = 0
    for id, block in reader:
        block_dict[id] = block
        while True:
            next_block = block_dict.pop(cur_id, None)
            if next_block is None:
                break

            yield cur_id, next_block
            cur_id += 1

    assert not block_dict, "invalid model db format"


class SQLFSReader(object):
    def __init__(self, conn, table):
        sql = "SELECT id, block FROM {};".format(table)
        self.raw_rs = conn.query(sql)
        self.rs = _build_ordered_reader(self.raw_rs)
        self.reader = iter(self.rs)
        self.buffer = b''

    def read(self, n):
        if n == 0:
            return b''

        if n < 0:
            raise ValueError("invalid number {}".format(n))

        while len(self.buffer) < n:
            new_buffer = next(self.reader, None)
            if new_buffer is None:
                break

            new_buffer = base64.b64decode(new_buffer[1])
            self.buffer += new_buffer

        read_length = min(n, len(self.buffer))
        result = self.buffer[0:read_length]
        self.buffer = self.buffer[read_length:]
        return result

    def close(self):
        self.raw_rs.close()

    def __enter__(self, *args, **kwargs):
        return self

    def __exit__(self, *args, **kwargs):
        self.close()


def _encode_metadata(metadata):
    metadata_json = json.dumps(metadata, cls=JSONEncoderWithFeatureColumn)
    if six.PY3:
        # make sure that metadata_json has no non-ascii characters
        metadata_json = bytes(metadata_json, encoding='utf-8')
    # encode length to an hex string
    # a string like 0x0000ffff (length 10) is able to represent int64.
    len_magic = "{0:#0{1}x}".format(len(metadata_json), 10)
    if six.PY3:
        len_magic = bytes(len_magic, encoding='utf-8')
    result = len_magic + metadata_json
    return result


def _read_metadata(reader):
    length = reader.read(10)
    length = int(length, 16)
    metadata_json = reader.read(length)
    return json.loads(metadata_json, cls=JSONDecoderWithFeatureColumn)


def write_with_generator_and_metadata(datasource, table, gen, metadata):
    """Write data into a table, the written data
    comes from the input generator and metadata.

    Args:
        datasource: string
            The connection string to connectDBMS.
        table: string
            The table name written.
        gen: Generator
            The generator to generate the data to insert
            into table.
        metadata: dict
            The metadata to be saved into the table. It would
            save in the row 0.
    """
    conn = connect_with_data_source(datasource)
    _drop_table_if_exists(conn, table)
    _create_table(conn, table)

    with SQLFSWriter(conn, table) as w:
        w.write(_encode_metadata(metadata))
        for d in gen():
            w.write(d)

    conn.close()


def read_metadata_from_db(datasource, table):
    """
    Read the metadata stored in the DBMS table.

    Args:
        datasource: string
            The connection string to connect DBMS.
        table: string
            The table name read.

    Returns: dict
        The metadata dict.
    """
    conn = connect_with_data_source(datasource)
    with SQLFSReader(conn, table) as r:
        metadata = _read_metadata(r)
    conn.close()
    return metadata


def read_with_generator_and_metadata(datasource, table, buff_size=256):
    """Read data from a table, this function returns
    a generator to yield the data, and the metadata dict.

    Args:
        datasource: string
            The connection string to connect DBMS.
        table: string
            The table name read.
        buff_size: int
            The buffer size to read data.

    Returns: tuple(Generator, dict)
        the generator yield row data of the table,
        and the model metadata dict.
    """
    conn = connect_with_data_source(datasource)
    r = SQLFSReader(conn, table)
    metadata = _read_metadata(r)

    def reader():
        while True:
            buffer = r.read(buff_size)
            if not buffer:
                break

            yield buffer

        r.close()
        conn.close()

    return reader, metadata
