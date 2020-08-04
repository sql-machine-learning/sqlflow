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

from abc import ABC, abstractmethod


class ResultSet(ABC):
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
        """Close the ResultSet explicitly, release any resource incurred by this query"""
        pass


class Connection(ABC):
    """Base class for DB connection

    Args:
        conn_uri: a connection uri in the schema://name:passwd@host/path?params format

    """
    def __init__(self, conn_uri):
        self.conn_uri = conn_uri

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
        return self._get_result_set(statement)

    def exec(self, statement):
        """Execute given statement and return True on success"""
        try:
            rs = self._get_result_set(statement)
            return rs.success()
        except:
            return False
        finally:
            rs.close()
