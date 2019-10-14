# How to Connect MySQL with SQLFlow

This tutorial explains how to connect SQLFlow with [MySQL](https://en.wikipedia.org/wiki/MySQL).

## Connect Existing MySQL Server

To connect an existed MySQL server instance, we need to configure a `datasource` string in the format of   
```
mysql://{username}:{password}@tcp({address})/{dbname}[?param1=value1&...&paramN=valueN]
```

In the above format,
- `username` indicates the user name, e.g `root`.
- `password` indicates the password, e.g. `root`.
- `address` indicates the ip address and the port number, e.g. `127.0.0.1:3306`.
- `dbname` indicates the database name, e.g. `iris`.
- `param1=value1` indicates additional configurations, e.g. `maxAllowedPacket=0`.

Putting these all together, we can construct a data source string like
```
mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0
```
Using the `datasource`, you may launch an all-in-one Docker container by running:  
```bash
> docker run --rm -p 8888:8888 sqlflow/sqlflow bash -c \
"sqlflowserver --datasource='mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

Open `localhost:8888` through a web browser, and you will find there are many SQLFlow tutorials, e.g. `iris-dnn.ipynb`. Please follow the tutorials and substitute the data for your use.

If you are running MySQL on remote, please be aware that MySQL only allows connections from localhost by default. The fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-mysql).

## Create a MySQL Server Locally for Testing

Our official Docker image has installed a `mysql` server. We can start it by `docker run --rm -it sqlflow/sqlflow bash`, then type `service mysql start` in the console. We should be able to see

```
$ service mysql start
 * Starting MySQL database server mysqld
No directory, logging in with HOME=/
                                                       [OK]
```

Then we can access the server by typing `mysql -uroot -proot`.
