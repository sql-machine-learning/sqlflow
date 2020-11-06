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


# NOTE: MySQL TEXT type can contain 65536 characters at most.
# We need to limit the max string length of each row.
MAX_LENGTH_TO_WRITE_PER_ROW = 32768


class SQLFSWriter(object):
    def __init__(self, conn, table):
        _drop_table_if_exists(conn, table)
        _create_table(conn, table)

        self.context_manager = buffered_db_writer(conn, table, ["id", "block"])
        self.writer = self.context_manager.__enter__()
        self.row_idx = 0
        self.buffer = b''

    def write(self, content):
        self.buffer += content
        start = 0
        end = MAX_LENGTH_TO_WRITE_PER_ROW
        length = len(self.buffer)
        while end <= length:
            self._write_impl(self.buffer[start:end])
            start = end
            end += MAX_LENGTH_TO_WRITE_PER_ROW

        if start > 0:
            self.buffer = self.buffer[start:]

    def _write_impl(self, content):
        block = base64.b64encode(content)
        if six.PY3 and isinstance(block, bytes):
            block = block.decode("utf-8")
        self.writer.write([self.row_idx, block])
        self.row_idx += 1

    def close(self):
        self.flush()
        # NOTE: __exit__ would close the self.writer
        self.context_manager.__exit__(None, None, None)

    def flush(self):
        if self.buffer:
            self._write_impl(self.buffer)
            self.buffer = b''

    def __enter__(self, *args, **kwargs):
        return self

    def __exit__(self, *args, **kwargs):
        self.close()


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


class BlockReader(object):
    def __init__(self, conn, table, row_buf_size):
        assert row_buf_size > 0, "row_buf_size must larger than 0"
        self.conn = conn
        self.table = table
        self.row_idx = 0
        self.row_buf_size = row_buf_size
        self.fragment_idx = 0
        self.fragments = []

    def _query_next(self):
        sql = "SELECT id, block FROM {} WHERE id>={} AND id<{};".format(
            self.table, self.row_idx, self.row_idx + self.row_buf_size)
        rs = self.conn.query(sql)
        fragments = list(rs)
        rs.close()
        fragments.sort(key=lambda f: f[0])
        assert len(fragments) <= self.row_buf_size, \
            "invalid sqlfs db table %s" % self.table

        i = 0
        for idx, _ in fragments:
            assert idx == self.row_idx + i, \
                "invalid sqlfs db table %s" % self.table
            i += 1
        return fragments

    def next_block(self):
        assert self.conn is not None, "read from a closed reader"

        if self.fragment_idx == len(self.fragments):
            if self.row_idx > 0 and len(self.fragments) < self.row_buf_size:
                return None

            self.fragments = self._query_next()
            self.row_idx += len(self.fragments)
            self.fragment_idx = 0

        if self.fragments:
            block = self.fragments[self.fragment_idx][1]
            self.fragment_idx += 1
            return block
        else:
            return None

    def close(self):
        self.conn = None


class SQLFSReader(object):
    def __init__(self, conn, table, row_buf_size=32):
        assert row_buf_size > 0, "row_buf_size must larger than 0"
        self.reader = BlockReader(conn, table, row_buf_size)
        self.buffer = b''

    def read(self, n):
        if n == 0:
            return b''

        if n < 0:
            raise ValueError("invalid number {}".format(n))

        while len(self.buffer) < n:
            new_buffer = self.reader.next_block()
            if new_buffer is None:
                break

            new_buffer = base64.b64decode(new_buffer)
            self.buffer += new_buffer

        read_length = min(n, len(self.buffer))
        result = self.buffer[0:read_length]
        self.buffer = self.buffer[read_length:]
        return result

    def close(self):
        self.reader.close()

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
    with connect_with_data_source(datasource) as conn:
        with SQLFSWriter(conn, table) as w:
            w.write(_encode_metadata(metadata))
            for d in gen():
                w.write(d)


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
    with connect_with_data_source(datasource) as conn:
        with SQLFSReader(conn, table) as r:
            metadata = _read_metadata(r)
            return metadata


def read_with_generator_and_metadata(datasource,
                                     table,
                                     buff_size=256,
                                     meta_only=False):
    """Read data from a table, this function returns
    a generator to yield the data, and the metadata dict.

    Args:
        datasource: string
            The connection string to connect DBMS.
        table: string
            The table name read.
        buff_size: int
            The buffer size to read data.
        meta_only: bool
            Only read the metadata.

    Returns: tuple(Generator, dict)
        the generator yield row data of the table,
        and the model metadata dict.
    """
    conn = connect_with_data_source(datasource)
    r = SQLFSReader(conn, table, 1 if meta_only else 32)
    metadata = _read_metadata(r)

    if meta_only:
        r.close()
        return None, metadata

    def reader():
        try:
            while True:
                buffer = r.read(buff_size)
                if not buffer:
                    break

                yield buffer
        finally:
            reader.close()

    def close():
        if not reader.is_closed:
            r.close()
            conn.close()
            reader.is_closed = True

    reader.is_closed = False
    reader.close = close

    return reader, metadata
