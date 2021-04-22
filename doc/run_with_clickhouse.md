# How to Connect Clickhouse with SQLFlow

This tutorial explains how to connect SQLFlow with [Clickhouse](https://clickhouse.tech/).

## Connect Existing Clickhouse Server

To connect an existed Clickhouse server instance, we need to configure a `datasource` string in the format of   
```
clickhouse://tcp({address:port})/{dbname}[?param1=value1&...&paramN=valueN]

clickhouse://tcp(192.168.31.114:9000)/iris?database=iris
clickhouse://tcp(192.168.31.114:9000)/iris?database=iris&username=default&othersettings...
```

In the above format,
- `username` indicates the user name, e.g `default`.
- `password` indicates the password, e.g. ``.
- `address` indicates the ip address and the port number, e.g. `127.0.0.1:9000`.
- `dbname` indicates the database name, e.g. `iris`.
- `param1=value1` indicates additional configurations, e.g. `connect_timeout=120` see at [Clickhouse Settings](https://clickhouse.tech/docs/en/operations/settings/).


Putting these all together, we can construct a data source string like
```
clickhouse://tcp(127.0.0.1:9000)/iris?username=hello&password=world&connect_timeout=120
```
Using the `datasource`, you may launch an all-in-one Docker container by running:  
```bash
> docker run --rm -p 8888:8888 sqlflow/sqlflow bash -c \
"sqlflowserver --datasource='clickhouse://tcp(127.0.0.1:9000)/iris?connect_timeout=0' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

Open `localhost:8888` through a web browser, and you will find there are many SQLFlow tutorials, e.g. `iris-dnn.ipynb`. Please follow the tutorials and substitute the data for your use.

If you are running Clickhouse on remote, please be aware that Clickhouse only allows connections from localhost by default. The fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-Clickhouse).

## Create a Clickhouse Server Locally for Testing
```
docker run -d --name ch-server --ulimit nofile=262144:262144 -p 8123:8123 -p 9000:9000 -p 9009:9009 yandex/clickhouse-server
```
### Get Demo Dataset from Mysql DB
```
docker run -p 3306:3306  sqlflow/sqlflow:mysql
```
OR 
1.  start a mysql server 
```
docker run -d -e MYSQL_ROOT_PASSWORD=password -p 3306:3306 --privileged=true mysql

```
2. load data in doc/datasets/*.sql
```
echo "Populate datasets ..."
for f in ./datasets/*; do
    echo "$f"
    mysql -uroot -h127.0.0.1 -pyourpass < "$f"
done
echo "Done."
```
3. create db in clickhouse
```
CREATE DATABASE iris_mysql ENGINE = MySQL('127.0.0.1:3306', 'iris', 'user', 'password');

CREATE DATABASE iris;
```
4. create table by select from iris_mysql
```
use iris

create table train engine=MergeTree() order by tuple() 
as select * from iris_mysql.train


create table test engine=MergeTree() order by tuple() 
as select * from iris_mysql.test

```
5. alter column type
```
alter table test modify column class String;
```

6. try it in sqlflow
```
./sqlflow -s 127.0.0.1:50051 -d 'clickhouse://tcp(192.168.31.114:9000)/iris?database=iris'

show databases

show tables

# train
SELECT * FROM iris.train TO TRAIN DNNClassifier WITH model.n_classes = 3, model.hidden_units = [10, 20] COLUMN sepal_length, sepal_width, petal_length, petal_width LABEL class INTO sqlflow_models.my_dnn_model

# predict
SELECT id, sepal_length,sepal_width,petal_length,petal_width,class FROM iris.test TO PREDICT iris.predict.class USING sqlflow_models.my_dnn_model;

# or all sepal_length+10  then do predict
SELECT id, toFloat32(plus(sepal_length,10)) as "sepal_length",sepal_width,petal_length,petal_width,class FROM iris.test TO PREDICT iris.predict.class USING sqlflow_models.my_dnn_model;


```

## Issues
Clickhouse_FIELD_TYPE_DICT in go and python is not fully covered.

The goal is make it run, then better.

The source files are in [clickhouse.py](../python/runtime/dbapi/clickhouse.py)
and [fields.go](../go/executor/fields.go)

If you got any errors related to data type in clickhouse,report it.
we will improve it later.