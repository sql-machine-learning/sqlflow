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

from odps import ODPS, tunnel
from runtime.dbapi.connection import Connection, ResultSet


class MaxComputeResultSet(ResultSet):
    """MaxCompute query result"""
    def __init__(self, instance, err=None):
        super().__init__()
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
            return self.column_info

        r = self._open_reader()
        self._column_info = [(col.name, col.type) for col in r._schema.columns]
        return self._column_info

    def _open_reader(self):
        if not self._reader:
            compress = tunnel.CompressOption.CompressAlgorithm.ODPS_ZLIB
            self._reader = self._instance.open_reader(tunnel=True,
                                                      compress_option=compress)
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
        super().__init__(conn_uri)
        self.params["database"] = self.params["curr_project"]
        # compose an endpoint, only keep the host and path and replace scheme
        endpoint = self.uripts._replace(scheme=self.params["scheme"],
                                        query="",
                                        netloc=self.uripts.hostname)
        self._conn = ODPS(self.uripts.username,
                          self.uripts.password,
                          project=self.params["database"],
                          endpoint=endpoint.geturl())

    def _parse_uri(self):
        return super()._parse_uri()

    def _get_result_set(self, statement):
        try:
            instance = self._conn.execute_sql(statement)
            return MaxComputeResultSet(instance)
        except Exception as e:
            return MaxComputeResultSet(None, str(e))

    def close(self):
        if self._conn:
            self._conn = None
