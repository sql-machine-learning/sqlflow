# Run SQLFlow Using Docker

SQLFlow releases an "all-in-one" Docker image that contains the SQLFlow server, MySQL
server, sample datasets, Jupyter Notebook server, and the SQLFlow plugin for Jupyter.

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
   docker run --rm -it -p 8888:8888 sqlflow/sqlflow
   ```

1. Open a web browser, go to `localhost:8888`, open `iris-dnn.ipynb` file, then you can
   follow the tutorial and run the SQL statements to run the training and prediction.

## Connect to Your Own Database


If you have your own database setup, below steps enables running a seperated container
that runs SQLFlow server and Jupyter Notebook, which connects to your own database.

### MySQL

Some sample data is already loaded inside the docker image, just type `service mysql start` to start MySQL instance, After that, let's test the installation by running a query in Jupyter Notebook. If you are using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`. If you are connecting to a remote database, please make sure to change `host.docker.internal:3306` to the remote address.

```
docker run -it -p 8888:8888 sqlflow/sqlflow \
bash -c "sqlflowserver --datasource='mysql://root:root@tcp(host.docker.internal:3306)/?maxAllowedPacket=0' &
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
```

If you are using Docker for Mac, please be aware the option `--database` where `host.docker.internal` translates to the host IP address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).

If you are running MySQL on remote, please be aware that MySQL only allows connections from localhost by default. Fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-mysql).

### Hive

Please refer to [run_with_hive.md](/doc/run_with_hive.md) for details on connecting with Hive.

