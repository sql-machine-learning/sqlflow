# How to Connect Hive with SQLFlow

This document is a tutorial on how SQLFlow connects Hive via [HiveServer2](https://cwiki.apache.org/confluence/display/Hive/HiveServer2+Overview).

## Connect Existing Hive Server

To connect an existing Hive server instance, we only need to configure a `datasource` string in the format of

``` text
hive://user:password@ip:port/dbname[?auth=<auth_mechanism>&session.<cfg_key1>=<cfg_value1>...&session<cfg_keyN>=valueN]
```

In the above format,

- `user:password` is the username and password of hiveserver2.
- `ip:port` is the endpoint which the hiveserver2 instance listened on.
- `dbname` is the default database name.
- `auth_mechanism` is the authentication mechanism of hiveserver2, can be `NOSASL` for unsecured transport or `PLAIN` for SASL transport.
- parameters with prefix `session.` is the session configuration of Hive Thrift API, such as `session.mapreduce.job.queuename=mr` implies `mapreduce.job.queuename=mr`.

You can find more details at [gohive](https://github.com/sql-machine-learning/gohive).

Using the `datasource` string, you can launch an all-in-one Docker container by running:

``` bash
docker run --rm -p 8888:8888 sqlflow/sqlflow bash -c \
"sqlflowserver &
SQLFLOW_DATASOURCE='hive://root:root@localhost:10000/iris' SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

Then you can open a web browser and go to `localhost:8888`. There are many SQLFlow tutorials, e.g. `tutorial_dnn_iris.ipynb`. You can follow the tutorials and substitute the data for your own use.

## Connect Standalone Hive Server for Testing

We also pack a standalone Hive server Docker image for testing.

### Connect Hive Server with NOSASL Transport

Launch your standalone hive server Docker container by running:

``` bash
> docker run -d -p 8888:8888 --name=hive sqlflow/gohive:dev
```

This implies settings in `hive-site.xml`:

``` text
hive.server2.authentication=NOSASL
```

Test SQLFlow by running the tutorials in Jupyter Notebook:

``` bash
> docker run --rm --net=container:hive sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='hive://root:root@localhost:10000/' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

## Connect Hive Server with PLAIN SASL Transport

This section would use the [PAM](https://cwiki.apache.org/confluence/display/Hive/Setting+Up+HiveServer2#SettingUpHiveServer2-PluggableAuthenticationModules(PAM)) authentication to do the demonstration.

Launch your standalone hive server Docker container with enable the PAM authentication:

``` bash
> docker run -d -e WITH_HS2_PAM_AUTH=ON -p 8888:8888 --name=hive sqlflow/gohive:dev
```

This implies settings in `hive-site.xml`:

``` text
hive.server2.authentication=PAM
hive.server2.authentication.pam.services=login,sshd
```

Test SQLFlow by running the tutorials in Jupyter Notebook:

``` bash
> docker run --rm --net=container:hive sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='hive://sqlflow:sqlflow@localhost:10000/?auth=PLAIN' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```
