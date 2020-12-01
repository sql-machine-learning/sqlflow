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

from runtime.dbapi.connection import Connection, ResultSet


class AlisaResultSet(ResultSet):
    """Alisa query result"""
    def _fetch(self, fetch_size):
        """Fetch given count of records in the result set

        Args:
            fetch_size: max record to retrive

        Returns:
            A list of records, each record is a list
            represent a row in the result set
        """
        pass

    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        pass

    def success(self):
        """Return True if the query is success"""
        return False

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


class AlisaConnection(Connection):
    """AlisaConnection connection, establish connection between client and alisa
    server via pyalisa

    Args:
        conn_uri: a connection uri in the schema://name:passwd@host/path?params
            format.

    """
    def __init__(self, conn_uri):
        pass

    def _get_result_set(self, statement):
        """Get the ResultSet for given statement

        Args:
            statement: the statement to execute

        Returns:
            A ResultSet object
        """
        pass

    def close(self):
        """
        Close the connection, implementation should support close multi-times
        """
        pass
