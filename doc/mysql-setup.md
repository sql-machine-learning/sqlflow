# Run MySQL Server and Client in Docker Containers

The document explains how to setup MySQL in our development environment.

## Run MySQL Server in a Docker Container

```bash
docker run --rm \
   -v /tmp/test1:/var/lib/mysql \
   --name mysql01 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST='%' \
   -p 3306:3306 \
   -d \
   mysql/mysql-server:8.0
```

- the `-v` option ensures that the database is saved on the host.  The default directory where MySQL saves the database is `/var/lib/mysql`. This directory can be configured in `/etc/mysql/my.cnf`, as explained in [this post](https://www.mkyong.com/mysql/where-does-mysql-stored-the-data-in-my-harddisk/).  By overlaying the directory `/tmp/test1` on the host to `/var/lib/mysql`, we "cheat" MySQL to save databases on the host.  So, we can kill the container and restart it, and the database is still there.

  Please be aware that the directory on the host must be empty the first time we run the above command; otherwise, MySQL would fail to initialize.  I figured out this problem after several failures using `docker logs`.

- the `-e` option sets the root password of MySQL to "root".  Feel free to set it to any password you like.

- the second `-e` options sets `MYSQL_ROOT_HOST` to a [wildcard](https://github.com/docker-library/mysql/issues/241#issuecomment-263011059) so to allow clients connecting to the server via TCP/IP as the user "root".  This trick works with MySQL 5.7 and 8.0, but not the most recent under-development version.

- the `--name` option names the container to `mysql01`, which can be used to refer to this container.

- the `-p` option maps the port 3306, on which the MySQL server listens, to the same port on the host, so that clients could connect to the server via TCP/IP.

## Run MySQL Client in the Server Container

```bash
docker exec -it mysql01 mysql -uroot -p
```

This command executes the command `mysql`, which is the command line tool of MySQL, in the container named `mysql01`.  

- The command line flags of `mysql` include `-u`, which specifies the username of MySQL, and `-p`, which makes MySQL prompts for the password.  For this example, we should type the password "root", which was set in the previous command.

- Please wait for a few seconds after the starting of the MySQL server container before we execute the client; otherwise, the startup of the client might fail.

- Once we get into the MySQL client, we can type SQL commands, e.g., 

  ```sql
  show databases;
  create database yi;
  ```

## Run Client in a Different Container on the Same Host

```bash
docker run --rm -it \
   -v /tmp/test1:/var/lib/mysql \
   mysql/mysql-server:8.0 \
   mysql -uroot -p
```

- The `-v` option maps the database directory on the host to the client container. This mapping is necessary because, by default, the client talks to the server via Unix socket `/var/lib/mysql/mysql.sock`, which is `/tmp/test1/mysql.sock` on the host.

## Run Client in a Container on a Remote Host

```bash
docker run \
   --rm -it \
   mysql/mysql-server:8.0 \
   mysql -h 192.168.1.3 -P 3306 -uroot -p
```

- the `-h` option tells the client where the server is running on.  In this example, the given IP is the one of the host where I ran the MySQL server container.

Please be aware that the above command works only if the server allows remote connections.

## Run Python Client in a Container

To write a Python client, we need to install the Python package `mysql-connector-python`.

```Dockerfile
FROM python:2.7
RUN pip install mysql-connector-python
```

Please be aware that some [documents](https://www.w3schools.com/python/python_mysql_getstarted.asp) says that we need to install `mysql-connector`.  I tried; but the `mysql.connector.connect` call failed with the error `mysql.connector.errors.NotSupportedError: Authentication plugin 'caching_sha2_password' is not supported`.

Build the Docker image:

```bash
docker build -t sqlflow .
```

Run the image:

```bash
docker run --rm -it sqlflow bash
```

and we can start Python and run the following Python code snippet

```
>>> import mysql.connector
>>> db = mysql.connector.connect(user='root', passwd='root', host='192.168.1.3')
>>> print(db)
<mysql.connector.connection_cext.CMySQLConnection object at 0x7fbab9f3fed0>
```

## Run a Go Client

In order to connect to a database, you need to import the database's driver first.

```
export GOPATH=$HOME/go
go get -u github.com/go-sql-driver/mysql
```

`go run` the following file

```go
package main

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"log"
)

func main() {
	testConfig := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Net:    "tcp",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testConfig.FormatDSN())
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
}
```
