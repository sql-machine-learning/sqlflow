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

from abc import ABCMeta, abstractmethod

import six
from six.moves.urllib.parse import parse_qs, urlparse


class ResultSet(six.Iterator):
    """Base class for DB query result, caller can iteratable this object
    to get all result rows"""
    def __init__(self):
        self._generator = None

    def __iter__(self):
        return self

    def _gen(self):
        fetch_size = 128
        while True:
            rows = self._fetch(fetch_size) or []
            for r in rows:
                yield r
            if len(rows) < fetch_size:
                break

    def __next__(self):
        if self._generator is None:
            self._generator = self._gen()
        return next(self._generator)

    @abstractmethod
    def _fetch(self, fetch_size):
        """Fetch given count of records in the result set

        Args:
            fetch_size: max record to retrive

        Returns:
            A list of records, each record is a list
            represent a row in the result set
        """
        pass

    def raw_column_info(self):
        return self.column_info()

    @abstractmethod
    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        pass

    @abstractmethod
    def success(self):
        """Return True if the query is success"""
        return False

    @abstractmethod
    def close(self):
        """Close the ResultSet explicitly, release any resource incurred by this query
        implementation should support close multi-times"""
        pass

    def error(self):
        """Get the error message if self.success()==False
        Returns:
            The error message
        """
        return ""


@six.add_metaclass(ABCMeta)
class Connection(object):
    """Base class for DB connection

    Args:
        conn_uri: a connection uri in the schema://name:passwd@host/path?params
            format.

    """
    def __init__(self, conn_uri):
        self.uristr = conn_uri
        self.uripts = self._parse_uri()
        self.driver = self.uripts.scheme
        self.params = parse_qs(
            self.uripts.query,
            keep_blank_values=True,
        )
        for k, l in self.params.items():
            if len(l) == 1:
                self.params[k] = l[0]

    def param(self, param_name, default_value=""):
        if not self.params:
            return default_value
        return self.params.get(param_name, default_value)

    def _parse_uri(self):
        """Parse the connection string into URI parts
        Returns:
            A ParseResult, different implementations should always pack
            the result into ParseResult
        """
        return urlparse(self.uristr)

    @abstractmethod
    def _get_result_set(self, statement):
        """Get the ResultSet for given statement

        Args:
            statement: the statement to execute

        Returns:
            A ResultSet object
        """
        pass

    def query(self, statement):
        """Execute given statement and return a ResultSet
        Typical usage will be:

        rs = conn.query("SELECT * FROM a;")
        result_rows = [r for r in rs]
        rs.close()

        Args:
            statement: the statement to execute

        Returns:
            A ResultSet object which is iteratable, each generated
            record in the iterator is a result-row wrapped by list
        """
        rs = self._get_result_set(statement)
        if rs.success():
            return rs
        else:
            raise Exception('Execute "%s" error\n%s' % (statement, rs.error()))

    def is_query(self, statement):
        """Return true if the statement is a query SQL statement."""
        s = statement.strip()
        s = s.upper()

        if s.startswith("SELECT") and s.find("INTO") == -1:
            return True
        if s.startswith("SHOW") and s.find("CREATE") >= 0 or s.find(
                "DATABASES") >= 0 or s.find("TABLES") >= 0:
            return True
        if s.startswith("DESC") or s.startswith("EXPLAIN"):
            return True
        return False

    def execute(self, statement):
        """Execute given statement and return True on success

        Args:
            statement: the statement to execute

        Returns:
            True on success, False otherwise
        """
        rs = None
        try:
            rs = self._get_result_set(statement)
            if rs.success():
                # NOTE(sneaxiy): must execute commit!
                # Otherwise, the `INSERT` statement
                # would have no effect even though
                # the connection is closed.
                self.commit()
                return True
            else:
                raise Exception('Execute "%s" error\n%s' %
                                (statement, rs.error()))
        finally:
            if rs is not None:
                rs.close()

    def get_table_schema(self, table_name):
        """Get table schema for given table

        Args:
            table_name: name of the table to get schema

        Returns:
            A list of (column_name, column_type) tuples
        """
        rs = self.query("SELECT * FROM %s limit 0" % table_name)
        column_info = rs.column_info()
        rs.close()
        return column_info

    @abstractmethod
    def close(self):
        """
        Close the connection, implementation should support
        close multi-times
        """
        pass

    def commit(self):
        pass

    def __del__(self):
        self.close()
