# Install SQLFlow Using Docker

SQLFlow releases an "all-in-one" Docker image that contains SQLFlow binary, MySQL
service, sample datasets loaded in the MySQL service, and jupyter notebook server.

You can use this Docker image for either local trying out or production deployment.

## Preparation

1. Install [Docker Community Edition](https://docs.docker.com/install/) on your PC/Macbook/Server.
1. Pull the latest SQLFlow "all-in-one" Docker image. Or you can also 
   build the Docker image from source code following [this guide](./build.md).

   ```
   docker pull sqlflow/sqlflow
   ```

## Try Out SQLFlow using Notebook

1. Type the below command to start the container:

   ```
   docker run -it -p 8888:8888 sqlflow/sqlflow
   ```

1. Open a web browser, go to `localhost:8888`, open `tutorial_dnn_iris.ipynb` file, then you can
   follow the tutorial and run the SQL statements to run the training and prediction.

## Connect to Your Own Database

If you have your own database setup, below steps enables running a seperated container
that runs SQLFlow server and Jupyter Notebook, which connects to your own database.

### MySQL

Follow steps in [example/datasets](https://github.com/sql-machine-learning/sqlflow/blob/develop/example/datasets) to import sample data.

After data is popularized in MySQL, let's test the installation by running a query in Jupyter Notebook. If you are using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`. If you are connecting to a remote database, please make sure to change `host.docker.internal:3306` to the remote address.

```
docker run -it -p 8888:8888 sqlflow/sqlflow \
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
docker run -it -p 8888:8888 sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='hive://root:root@host.docker.internal:10000/' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
```

If you are using Docker for Mac, please be aware the option `--database` where `host.docker.internal` translates to the host IP address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).
