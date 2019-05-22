def connect(driver, database, user, passwd, host, port):
    if driver == "mysql":
        return mysql.connector.connect(user=user,
                                       passwd=passwd,
                                       database=database,
                                       host=host,
                                       port=port)
    elif driver == "sqllite3":
        return sqlite3.connect(database)
    elif driver == "hive":
        return impala.dbapi.connect(user=user,
                                    password=passwd,
                                    database=database,
                                    host=host,
                                    port=port)
    raise ValueError("unrecognized database driver: {{.Driver}}")
