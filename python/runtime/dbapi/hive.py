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
    from impala.dbapi import connect
except:  # noqa E722
    pass
from runtime.dbapi.connection import Connection, ResultSet


class HiveResultSet(ResultSet):
    def __init__(self, cursor, err=None):
        super(HiveResultSet, self).__init__()
        self._cursor = cursor
        self._column_info = None
        self._raw_column_info = None
        self._err = err

    def _fetch(self, fetch_size):
        return self._cursor.fetchmany(fetch_size)

    def _fill_column_info(self):
        columns = []
        raw_columns = []
        for desc in self._cursor.description:
            name = desc[0].split('.')[-1]
            columns.append((name, desc[1]))
            raw_columns.append((desc[0], desc[1]))
        self._column_info = columns
        self._raw_column_info = raw_columns

    def raw_column_info(self):
        if self._raw_column_info is None:
            self._fill_column_info()

        return self._raw_column_info

    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """

        if self._column_info is None:
            self._fill_column_info()

        return self._column_info

    def success(self):
        """Return True if the query is success"""
        return self._cursor is not None

    def error(self):
        return self._err

    def close(self):
        """Close the ResultSet explicitly, release any
        resource incurred by this query"""
        if self._cursor:
            self._cursor.close()
            self._cursor = None


class HiveConnection(Connection):
    """Hive connection

    conn_uri: uri in format:
        hive://usr:pswd@hiveserver:10000/mydb?auth=PLAIN&session.mapred=mr
        All params start with 'session.' will be treated as session
        configuration
    """
    def __init__(self, conn_uri):
        super(HiveConnection, self).__init__(conn_uri)
        self.driver = "hive"
        self.params["database"] = self.uripts.path.strip("/")
        self._conn = connect(user=self.uripts.username,
                             password=self.uripts.password,
                             database=self.params["database"],
                             host=self.uripts.hostname,
                             port=self.uripts.port,
                             auth_mechanism=self.params.get("auth"))
        self._session_cfg = dict([(k, v) for (k, v) in self.params.items()
                                  if k.startswith("session.")])

    def _get_result_set(self, statement):
        cursor = self._conn.cursor(user=self.uripts.username,
                                   configuration=self._session_cfg)
        try:
            cursor.execute(statement.rstrip(";"))
            return HiveResultSet(cursor)
        except Exception as e:
            cursor.close()
            return HiveResultSet(None, str(e))

    def cursor(self):
        """Get a cursor on the connection
        We insist not to use the low level api like cursor.
        Instead, we can directly use query/exec
        """
        return self._conn.cursor()

    def commit(self):
        return self._conn.commit()

    def close(self):
        if self._conn:
            self._conn.close()
            self._conn = None
