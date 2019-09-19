# Run SQLFlow with Hive via HiveServer2

This is a tutorial on how to run SQLFlow which connects to the hive server2.

For the most production environment, the system administrators may setup hive server with [authentication configuration](https://cwiki.apache.org/confluence/display/Hive/Setting+Up+HiveServer2#SettingUpHiveServer2-Authentication/SecurityConfiguration): e.g. KERBEROS, LDAP, PAM or CUSTOM.

## Connect Hive Server wih No SASL

Launch your standalone hive server Docker container by running:

``` bash
> docker run -d -p 8888:8888 --name=hive sqlflow/gohive:dev python3 -m http.server 8899
```

This implies settings in `hive-site.xml`:

``` text
hive.server2.authentication = NOSASL
```

Test SQLFlow by running a query in Jupyter Notebook

``` bash
> docker run --rm --net=container:hive sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='hive://root:root@localhost:10000/' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

## Connect Hive Server with PLAIN SASL

This section would use the [PAM](https://cwiki.apache.org/confluence/display/Hive/Setting+Up+HiveServer2#SettingUpHiveServer2-PluggableAuthenticationModules(PAM)) authentication to do the demonstration.

Launch your standalone hive server Docker container with enable the PAM authentication:

``` bash
> docker run -d -e WITH_HS2_PAM_AUTH=ON -p 8888:8888 --name=hive sqlflow/gohive:dev python3 -m http.server 8899
```

This implies settings in `hive-site.xml`:

``` text
hive.server2.authentication = PAM
```

Test SQLFlow by running a query in Jupyter Notebook:

``` bash
> docker run --rm --net=container:hive sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='hive://sqlflow:sqlflow@localhost:10000/?auth=PLAIN' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```
