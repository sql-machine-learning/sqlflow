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
# limitations under the License

try:
    from odps import ODPS, tunnel
    COMPRESS_ODPS_ZLIB = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
except:  # noqa: E722
    COMPRESS_ODPS_ZLIB = None

from runtime.dbapi.connection import Connection, ResultSet
from six.moves.urllib.parse import parse_qs, urlparse


class MaxComputeResultSet(ResultSet):
    """MaxCompute query result"""
    def __init__(self, instance, err=None):
        super(MaxComputeResultSet, self).__init__()
        self._instance = instance
        self._column_info = None
        self._err = err
        self._reader = None
        self._read_count = 0

    def _fetch(self, fetch_size):
        r = self._open_reader()
        count = min(fetch_size, r.count - self._read_count)
        rows = [[f[1] for f in row]
                for row in r[self._read_count:self._read_count + count]]
        self._read_count += count
        return rows

    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        if self._column_info is not None:
            return self._column_info

        r = self._open_reader()
        self._column_info = [(col.name, str(col.type).upper())
                             for col in r._schema.columns]
        return self._column_info

    def _open_reader(self):
        if not self._reader:
            self._reader = self._instance.open_reader(
                tunnel=True, compress_option=COMPRESS_ODPS_ZLIB)
        return self._reader

    def success(self):
        """Return True if the query is success"""
        return self._instance is not None and self._instance.is_successful()

    def error(self):
        return self._err

    def close(self):
        if self._reader:
            if hasattr(self._reader, "close"):
                self._reader.close()
            self._reader = None
            self._instance = None

    def __del__(self):
        self.close()


class MaxComputeConnection(Connection):
    """MaxCompute connection, this class uses ODPS object to establish
    connection with maxcompute

    Args:
        conn_uri: uri in format:
        maxcompute://access_id:access_key@service.com/api?curr_project=test_ci&scheme=http
    """
    def __init__(self, conn_uri):
        super(MaxComputeConnection, self).__init__(conn_uri)
        user, pwd, endpoint, proj = MaxComputeConnection.get_uri_parts(
            conn_uri)
        self.driver = "maxcompute"
        self.params["database"] = proj
        self.endpoint = endpoint
        self._conn = ODPS(user, pwd, project=proj, endpoint=endpoint)

    @staticmethod
    def get_uri_parts(uri):
        """Get username, password, endpoint, projectfrom given uri

        Args:
            uri: a valid maxcompute connection uri

        Returns:
            A tuple (username, password, endpoint, project)
        """
        uripts = urlparse(uri)
        params = parse_qs(uripts.query)
        # compose an endpoint, only keep the host and path and replace scheme
        endpoint = uripts._replace(scheme=params.get("scheme", ["http"])[0],
                                   query="",
                                   netloc=uripts.hostname)
        endpoint = endpoint.geturl()
        return (uripts.username, uripts.password, endpoint,
                params.get("curr_project", [""])[0])

    def _get_result_set(self, statement):
        try:
            instance = self._conn.execute_sql(statement)
            return MaxComputeResultSet(instance)
        except Exception as e:
            raise e

    def close(self):
        if self._conn:
            self._conn = None

    def get_table_schema(self, table_name):
        schema = self._conn.get_table(table_name).schema
        return [(c.name, str(c.type).upper()) for c in schema.columns]

    def write_table(self,
                    table_name,
                    rows,
                    compress_option=COMPRESS_ODPS_ZLIB):
        """Append rows to given table, this is a driver specific api

        Args:
            table_name: the table to write
            rows: list of rows, each row is a data tuple,
                like [(1,True,"ok"),(2,False,"bad")]
            compress_options: the compress options defined in
                tunnel.CompressOption.CompressAlgorithm
        """
        self._conn.write_table(table_name,
                               rows,
                               compress_option=compress_option)
