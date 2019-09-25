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

## Try Out SQLFlow Using Notebook

1. Type the below command to start the container:

   ```
   docker run --rm -it -p 8888:8888 sqlflow/sqlflow
   ```

1. Open a web browser, go to `localhost:8888`, open `iris-dnn.ipynb` file, then you can
   follow the tutorial and run the SQL statements to run the training and prediction.

## Connect to Your Own Database


If you have your own database setup, below steps enables running a seperated
SQLFlow server and Jupyter Notebook which connects to your own database service.

For MySQL, please refer to [run_with_mysql.md](/doc/run_with_mysql.md).

For Hive, please refer to [run_with_hive.md](/doc/run_with_hive.md).

For MaxCompute, please refer to [run_with_maxcompute.md](/doc/run_with_maxcompute.md).
