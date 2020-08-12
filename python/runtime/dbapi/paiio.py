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

from __future__ import absolute_import

import re

from runtime.dbapi.connection import Connection, ResultSet

try:
    import paiio
except Exception:  # noqa: E722
    pass


class PaiIOResultSet(ResultSet):
    def __init__(self, reader, err=None):
        super(PaiIOResultSet, self).__init__()
        self._reader = reader
        self._column_info = None
        self._err = err

    def _fetch(self, fetch_size):
        try:
            return self._reader.read(num_records=fetch_size)
        except Exception:  # noqa: E722
            pass

    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        if self._column_info is not None:
            return self._column_info

        schema = self._reader.get_schema()
        columns = [(c['colname'], str(c['typestr']).upper()) for c in schema]
        self._column_info = columns
        return self._column_info

    def success(self):
        """Return True if the query is success"""
        return self._reader is not None

    def error(self):
        return self._err

    def close(self):
        """
        Close the ResultSet explicitly, release any resource incurred
        by this query
        """
        if self._reader:
            self._reader.close()
            self._reader = None

    def __del__(self):
        self.close()


class PaiIOConnection(Connection):
    """PaiIOConnection emulate a connection for paiio,
    currently only support full-table reading. That means
    we can't filter the data, join the table and so on.
    The only supported query statement is `None`. The scheme
    part of the uri can be 'paiio' or 'odps'

    Typical use is:
    con = PaiIOConnection("paiio://db/tables/my_table")
    res = con.query(None)
    rows = [r for r in res]
    """
    def __init__(self, conn_uri):
        super(PaiIOConnection, self).__init__(conn_uri)
        # (TODO: lhw) change driver to paiio
        self.driver = "pai_maxcompute"
        match = re.findall(r"\w+://\w+/tables/(.+)", conn_uri)
        if len(match) < 1:
            raise ValueError("Should specify table in uri with format: "
                             "paiio://db/tables/table?param_a=a&param_b=b")
        self.params["table"] = conn_uri.replace("paiio://", "odps://")
        self.params["slice_id"] = self.params.get("slice_id", 0)
        self.params["slice_count"] = self.params.get("slice_count", 1)

    def _get_result_set(self, statement):
        if statement is not None:
            raise ValueError("paiio only support full table read,"
                             "so you need to pass statement with None.")
        try:
            reader = paiio.TableReader(self.params["table"],
                                       slice_id=self.params["slice_id"],
                                       slice_count=self.params["slice_count"])
            return PaiIOResultSet(reader, None)
        except Exception as e:
            return PaiIOResultSet(None, str(e))

    def get_table_schema(self, full_uri):
        """Get schema of given table, caller need to supply the full
        uri for paiio table, this is slight different with other connections.
        """
        return PaiIOConnection.get_schema(full_uri)

    def query(self, statement=None):
        return super(PaiIOConnection, self).query(statement)

    @staticmethod
    def get_table_row_num(table_uri):
        """Get row number of given table

        Args:
            table_uri: the full uri for the table to get row from

        Return:
            Number of rows in the table
        """
        reader = paiio.TableReader(table_uri)
        row_num = reader.get_row_count()
        reader.close()
        return row_num

    @staticmethod
    def get_schema(table_uri):
        """Get schema of the given table

        Args:
            table_uri: the full uri for the table to get row from

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        rs = PaiIOConnection(table_uri).query()
        col_info = rs.column_info()
        rs.close()
        return col_info

    def close(self):
        pass
