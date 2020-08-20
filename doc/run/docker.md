# Run SQLFlow Using Docker

SQLFlow releases several Docker images that contains the SQLFlow server, MySQL
server, sample datasets, Jupyter Notebook server, and the SQLFlow plugin for Jupyter.

You can use these Docker images for either local trying out or production deployment.

## Preparation

1. Install [Docker Community Edition](https://docs.docker.com/install/) on your PC/Macbook/Server.
1. Pull the latest SQLFlow Docker images. Or you can also 
   build the Docker image from source code following [this guide](../build.md).

   ```
   docker pull sqlflow/sqlflow
   docker pull sqlflow/sqlflow:mysql
   docker pull sqlflow/sqlflow:jupyter
   ```

## Try Out SQLFlow Using Notebook

1. Type the below command to start three containers to start a MySQL server, SQLFlow server and a Jupyter notebook server.

   ```
   docker run --name=sqlflow-mysql -d -p 8888:8888 sqlflow/sqlflow:mysql
   docker run --net=container:sqlflow-mysql -d sqlflow/sqlflow:lateset sqlflowserver
   docker run --net=container:sqlflow-mysql -d sqlflow/sqlflow:jupyter
   ```
1. You can also use a specified version (e.g. `v0.4.0`) of the SQLFlow server by changing the second line above to `docker run --net=container:sqlflow-mysql -d sqlflow/sqlflow:v0.4.0 sqlflowserver`.
1. Open a web browser, go to `localhost:8888`, open any tutorial notebook like  `iris-dnn.ipynb` file, then you can follow the tutorial and run the SQL statements to run the training and prediction.

## Connect to Your Own Database


If you have your own database setup, below steps enables running a separated
SQLFlow server and Jupyter Notebook which connects to your own database service.

For MySQL, please refer to [run_with_mysql](../run_with_mysql.md).

For Hive, please refer to [run_with_hive](../run_with_hive.md).

For MaxCompute, please refer to [run_with_maxcompute](../run_with_maxcompute.md).
