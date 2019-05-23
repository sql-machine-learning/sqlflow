

def connect(driver, database, user, password, host, port):
    if driver == "mysql":
        from mysql.connector import connect
        return connect(user=user,
                       passwd=password,
                       database=database,
                       host=host,
                       port=port)
    elif driver == "sqlite3":
        from sqlite3 import connect
        return connect(database)
    elif driver == "hive":
        from impala.dbapi import connect
        return connect(user=user,
                       passwd=password,
                       database=database,
                       host=host,
                       port=port)

    raise ValueError("unrecognized database driver: %s" % driver)


def execute(driver, conn, statement):
    cursor = conn.cursor()
    cursor.execute(statement)

    if driver == "hive":
        field_names = None if cursor.description is None \
            else [i[0][i[0].find('.') + 1:] for i in cursor.description]
    else:
        field_names = None if cursor.description is None \
            else [i[0] for i in cursor.description]

    rows = cursor.fetchall()
    field_columns = list(map(list, zip(*rows))) if len(rows) > 0 else None

    return field_names, field_columns


def insert_values(driver, conn, table_name, table_schema, values):
    if driver == "mysql":
        statement = '''insert into {} ({}) values()'''.format(
            table_name,
            ", ".join([n for n, t in table_schema]),
            ", ".join(["%s"] * len(table_schema))
        )
    elif driver == "sqlite3":
        statement = '''insert into {} ({}) values({})'''.format(
            table_name,
            ", ".join([n for n, t in table_schema]),
            ", ".join(["?"] * len(table_schema))
        )
    elif driver == "hive":
        statement = '''insert into table {} ({}) values()'''.format(
            table_name,
            ", ".join([n for n, t in table_schema]),
            ", ".join(["%s"] * len(table_schema))
        )
    else:
        raise ValueError("unrecognized database driver: %s" % driver)

    cursor = conn.cursor()
    cursor.executemany(statement, values)
    conn.commit()

    return cursor
