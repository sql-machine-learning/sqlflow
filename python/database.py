import mysql.connector

def type_to_string(x):
    if isinstance(x, int):
        return 'INT'
    elif isinstance(x, float):
        return 'FLOAT'
    else:
        assert(False)


def create_table(user, passwd, host, database_name, table_name, data):
    TABLE_FIELD = "(" + ",".join([x[0] for x in data]) + ")"
    TABLE_SCHEMA = "(" + ",".join([x[0] + " " + type_to_string(x[1][0]) for x in data]) + ")"
    
    print("Cleaning up database " + database_name)
    db = mysql.connector.connect(user=user, passwd=passwd, host=host)
    cursor = db.cursor()
    cursor.execute("SHOW DATABASES")
    for x in cursor:
        if database_name == x[0]:
            db.cursor().execute("DROP DATABASE " + database_name)
            break
    
    print("Creating database {}".format(database_name))
    db.cursor().execute("CREATE DATABASE " + database_name)
    
    print("Creating table {}".format(table_name))
    db = mysql.connector.connect(user="root", passwd="root", host="localhost", database=database_name)
    cursor = db.cursor()
    cursor.execute("CREATE TABLE {} {}".format(table_name, TABLE_SCHEMA))

    print("Inserting data")
    sql = "INSERT INTO {} {} VALUES {}".format(table_name, TABLE_FIELD, "(%s, %s, %s, %s, %s)")
    # convert to row based
    val = map(list, zip(*[x[1] for x in data]))

    cursor.executemany(sql, val)
    db.commit()

def load_data(user, passwd, host, database_name, sql_command):
    db = mysql.connector.connect(user=user, passwd=passwd, host=host, database=database_name)
    cursor = db.cursor()
    
    cursor.execute(sql_command)
    
    field_names = [i[0] for i in cursor.description]
    columns = map(list, zip(*cursor.fetchall()))

    return field_names, columns


