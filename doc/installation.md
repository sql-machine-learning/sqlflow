# Installation

SQLFlow is currently under active development. For those who are interested in trying
it out, we have provided the instructions and demo. Play around with it. Any bug report and issue is welcome. :)


## Preparation

1. Install [Docker Community Edition](https://docs.docker.com/install/) on your Macbook.
1. Pull the latest SQLFlow "all-in-one" Docker image, which contains pre-built SQLFlow
   binary, sample datasets (under `example/datasets`), and jupyter notebook.

   ```
   docker pull sqlflow/sqlflow:latest
   ```

## Running Your First SQLFlow Query

1. Simply type the below command to start the service:

   ```
   docker run -it -p 8888:8888 sqlflow/sqlflow:latest
   ```

1. Open a web browser, go to `localhost:8888` .  Select the "New" drop-down menu on the right side, and open the "Python 3" development environment in a new Notebook cell (also in a new tab). In the new cell, type in below SELECT statement to fetch 5 records from train table in Iris database. 

   ```
   %%sqlflow
   select * from iris.train limit 5;
   ```

1. Now you've successfully tested SQLFlow installation and written some SQL from Jupyter Notebook. Just as shown in the Quick Overview, you can continue your [SQLFlow journey](demo.md) in the command line setting. Also feel free to check out more [SQLFlow examples](/example/jupyter/example.ipynb) if you are new to Jupyter Notebook.

## Use Your Own Database

If you have your own database setup, below steps enables running a seperated container
that runs SQLFlow server and Jupyter Notebook, which connects to your own database.

### MySQL

Follow steps in [example/datasets](https://github.com/sql-machine-learning/sqlflow/blob/develop/example/datasets) to import sample data.

After data is popularized in MySQL, let's test the installation by running a query in Jupyter Notebook. If you are using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`. If you are connecting to a remote database, please make sure to change `host.docker.internal:3306` to the remote address.

```
docker run -it -p 8888:8888 sqlflow/sqlflow:latest \
bash -c "sqlflowserver --datasource='mysql://root:root@tcp(host.docker.internal:3306)/?maxAllowedPacket=0' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
```

If you are using Docker for Mac, please be aware the option `--database` where `host.docker.internal` translates to the host IP address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).

If you are running MySQL on remote, please be aware that MySQL only allows connections from localhost by default. Fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-mysql).

### Hive

Start your standalone Hive server with populated data by running

```
docker run -d -p 10000:10000 -p 10002:10002 -p 8040:8040 -p 8042:8042 -p 9864:9864 -p 9866:9866 -p 9867:9867 -p 9870:9870 -p 8020:8020 -p 8899:8899 sqlflow/gohive:dev python3 -m http.server 8899
```

Test the installation by running a query in Jupyter Notebook. If you are using Docker for Linux, please change `host.docker.internal:10000` to `localhost:10000`. If you are connecting to a remote database, please make sure to change `host.docker.internal:10000` to the remote address.

```
docker run -it -p 8888:8888 sqlflow/sqlflow:latest \
bash -c "sqlflowserver --datasource='hive://root:root@host.docker.internal:10000/' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
```

If you are using Docker for Mac, please be aware the option `--database` where `host.docker.internal` translates to the host IP address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).
