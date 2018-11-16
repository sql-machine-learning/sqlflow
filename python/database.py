import mysql.connector

def load_data(user, passwd, host, database_name, sql_command):
    db = mysql.connector.connect(user=user, passwd=passwd, host=host, database=database_name)
    cursor = db.cursor()
    
    cursor.execute(sql_command)
    
    field_names = [i[0] for i in cursor.description]
    columns = map(list, zip(*cursor.fetchall()))

    return field_names, columns


