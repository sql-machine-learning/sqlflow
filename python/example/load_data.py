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

    print("Done")

USER = "root"
PASSWORD = "root"
HOST = "localhost"
DATABASE = "yang"
TABLE = "irisis"

DATA = [("sepal_length", [5.1, 5.0, 6.4]),
        ("sepal_width", [3.3, 2.3, 2.8]),
        ("petal_length", [1.7, 3.3, 5.6]),
        ("petal_width", [0.5, 1.0, 2.2]),
        ("species", [0, 1, 2])]

create_table(USER, PASSWORD, HOST, DATABASE, TABLE, DATA)
