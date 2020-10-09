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

import re

from runtime.dbapi.connection import Connection, ResultSet
from six.moves.urllib.parse import ParseResult

# NOTE: use MySQLdb to avoid bugs like infinite reading:
# https://bugs.mysql.com/bug.php?id=91971
try:
    from MySQLdb import connect
except:  # noqa E722
    pass

try:
    import MySQLdb.constants.FIELD_TYPE as MYSQL_FIELD_TYPE
    # Refer to
    # http://mysql-python.sourceforge.net/MySQLdb-1.2.2/public/MySQLdb.constants.FIELD_TYPE-module.html # noqa: E501
    MYSQL_FIELD_TYPE_DICT = {
        MYSQL_FIELD_TYPE.TINY: "TINYINT",  # 1
        MYSQL_FIELD_TYPE.LONG: "INT",  # 3
        MYSQL_FIELD_TYPE.FLOAT: "FLOAT",  # 4
        MYSQL_FIELD_TYPE.DOUBLE: "DOUBLE",  # 5
        MYSQL_FIELD_TYPE.LONGLONG: "BIGINT",  # 8
        MYSQL_FIELD_TYPE.NEWDECIMAL: "DECIMAL",  # 246
        MYSQL_FIELD_TYPE.BLOB: "TEXT",  # 252
        MYSQL_FIELD_TYPE.VAR_STRING: "VARCHAR",  # 253
        MYSQL_FIELD_TYPE.STRING: "CHAR",  # 254
    }
except:  # noqa: E722
    MYSQL_FIELD_TYPE_DICT = {}


class MySQLResultSet(ResultSet):
    def __init__(self, cursor, err=None):
        super(MySQLResultSet, self).__init__()
        self._cursor = cursor
        self._column_info = None
        self._err = err

    def _fetch(self, fetch_size):
        return self._cursor.fetchmany(fetch_size)

    def column_info(self):
        """Get the result column meta, type in the meta maybe DB-specific

        Returns:
            A list of column metas, like [(field_a, INT), (field_b, STRING)]
        """
        if self._column_info is not None:
            return self._column_info

        columns = []
        for desc in self._cursor.description or []:
            # NOTE: MySQL returns an integer number instead of a string
            # to represent the data type.
            typ = MYSQL_FIELD_TYPE_DICT.get(desc[1])
            if typ is None:
                raise ValueError("unsupported data type of column {}".format(
                    desc[0]))
            columns.append((desc[0], typ))
        self._column_info = columns
        return self._column_info

    def success(self):
        """Return True if the query is success"""
        return self._cursor is not None

    def error(self):
        return self._err

    def close(self):
        """
        Close the ResultSet explicitly, release any resource incurred
        by this query
        """
        if self._cursor:
            self._cursor.close()
            self._cursor = None


class MySQLConnection(Connection):
    def __init__(self, conn_uri):
        super(MySQLConnection, self).__init__(conn_uri)
        self.driver = "mysql"
        self.params["database"] = self.uripts.path.strip("/")
        self._conn = connect(user=self.uripts.username,
                             passwd=self.uripts.password,
                             db=self.params["database"],
                             host=self.uripts.hostname,
                             port=self.uripts.port)

    def _parse_uri(self):
        # MySQL connection string is a DataSourceName(DSN),
        # the username, passwd can be any character.
        pattern = r"^(\w+)://(.*):(.*)@tcp\(([.a-zA-Z0-9\-]*):?([0-9]*)\)/(\w*)(\?.*)?$"  # noqa: W605, E501
        found_result = re.findall(pattern, self.uristr)
        scheme, user, passwd, host, port, db, config = found_result[0]
        netloc = "{}:{}@{}:{}".format(user, passwd, host, port or 3306)
        return ParseResult(scheme, netloc, db, "", config.lstrip("?"), "")

    def _get_result_set(self, statement):
        cursor = self._conn.cursor()
        try:
            cursor.execute(statement)
            return MySQLResultSet(cursor)
        except Exception as e:
            cursor.close()
            return MySQLResultSet(None, str(e))

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
