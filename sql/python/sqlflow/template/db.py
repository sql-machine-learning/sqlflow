from __future__ import absolute_import
from __future__ import division
from __future__ import print_function
import abc
import six


@six.add_metaclass(abc.ABCMeta)
class DataBase(object):
    """A database interface """

    def __init__(self, database, user, password, host, port):
        self._database = database
        self._user = user
        self._password = password
        self._host = host
        self._port = port

        self._db = self._connect()

    def close(self):
        self._db.close()

    def execute(self, query):
        """ Execute the query
        Args:
            query(str): the query to execute

        Returns:
            cursor(cursor):
        """
        cursor = self._db.cursor()
        cursor.execute(query)
        return cursor

    def execute_many(self, query, values):
        """ Execute the query with values
        Args:
            query(str): the query to execute
            values(list(tuple)): the values

        Returns:
            cursor(cursor)
        """
        cursor = self._db.cursor()
        cursor.executemany(query, values)
        return cursor

    def create_table(self, table_name, table_schema):
        """ Create table
        Args:
            table_name(str): name of table
            table_schema(list(tuple)): schema of table

        Returns:
            success if no error thrown out
        """
        statement = self._gen_create_table_statement(table_name, table_schema)
        self.execute(statement)

    def insert_values(self, table_name, table_schema, values):
        """ Insert values to table
        Args:
            table_name(str): name of table
            table_schema(list(str)): schema of table
            values(list(tuple)): the value to insert

        Returns:
            success if no error thrown out
        """
        statement = self._gen_insert_table_statement(table_name, table_schema)
        self.execute_many(statement, values)

    def query_select(self, statement):
        """ Select fields from table simply
        Args:
            statement(str): select statement

        Return:
            Returns:
            field_names(list(str)): the names of fields
            field_data(list): return data of column-style
        """
        cursor = self.execute(statement)
        return self._retrieve_fields_and_value_from_cursor(cursor)

    def _connect(self):
        raise NotImplementedError("DataBase.connect")

    def _gen_create_table_statement(self, table_name, table_schema):
        # TODO replace the type of fields according to different kind of database
        return '''CREATE TABLE `{}` ({})'''.format(
            table_name, ", ".join(["%s %s" % (n, t) for n, t in table_schema]))

    def _gen_insert_table_statement(self, table_name, table_schema):
        raise NotImplementedError("DataBase._gen_insert_table_statement")

    def _retrieve_fields_and_value_from_cursor(self, cursor):
        raise NotImplementedError("DataBase._retrieve_fields_and_value_from_cursor")


class MySQLDataBase(DataBase):

    def _connect(self):
        from mysql.connector import connect
        return connect(user=self._user,
                       passwd=self._password,
                       database=self._database,
                       host=self._host,
                       port=self._port)

    def _retrieve_fields_and_value_from_cursor(self, cursor):
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]
        field_columns = list(map(list, zip(*cursor.fetchall())))

        return field_names, field_columns

    def _gen_insert_table_statement(self, table_name, table_schema):
        return '''insert into {} ({}) values()'''.format(
            table_name,
            ", ".join([n for n, t in table_schema]),
            ", ".join(["%s"] * len(table_schema))
        )


class SQLite3DataBase(DataBase):

    def _connect(self):
        from sqlite3 import connect
        return connect(self._database)

    def _retrieve_fields_and_value_from_cursor(self, cursor):
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]
        field_columns = list(map(list, zip(*cursor.fetchall())))

        return field_names, field_columns

    def _gen_insert_table_statement(self, table_name, table_schema):
        return '''insert into {} ({}) values({})'''.format(
            table_name,
            ", ".join([n for n, t in table_schema]),
            ", ".join(["?"] * len(table_schema))
        )


class HiveDataBase(DataBase):

    def _connect(self):
        from impala.dbapi import connect
        return connect(user=self._user,
                       passwd=self._password,
                       database=self._database,
                       host=self._host,
                       port=self._port)

    def _retrieve_fields_and_value_from_cursor(self, cursor):
        field_names = None if cursor.description is None \
            else [i[0][i[0].find('.')+1:] for i in cursor.description]
        field_columns = list(map(list, zip(*cursor.fetchall())))

        return field_names, field_columns

    def _gen_insert_table_statement(self, table_name, table_schema):
        raise NotImplementedError("not support")


def get_database(driver, database, user, password, host, port):
    if driver == "mysql":
        return MySQLDataBase(database, user, password, host, port)
    elif driver == "sqllite3":
        return SQLite3DataBase(database, user, password, host, port)
    elif driver == "hive":
        return HiveDataBase(database, user, password, host, port)

    raise ValueError("unrecognized database driver: %s" % driver)
