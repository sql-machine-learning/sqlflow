# Quick start

SQLFlow is currently under active development. For those who are interested in trying
it out, we have provided several demos. Play around with it. Any bug report and
issue are welcomed. :)

## Setup

1. Install [Docker](https://docs.docker.com/install/).
1. Set up a MySQL server following [example/datasets/README.md](/example/datasets/README.md).
1. Pull the latest SQLFlow Docker image: `docker pull sqlflow/sqlflow:latest`.

## Demo 1: Jupyter Notebook

1. Start a Docker container that runs sqlflowserver and Jupyter Notebook. If you are
   using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`.

   ```
   docker run --rm -it -p 8888:8888 sqlflow/sqlflow:latest \
   bash -c "sqlflowserver --db_user root --db_password root --db_address host.docker.internal:3306 &
   SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
   ```

   If you are using Docker for Mac, please be aware the option `--db_address host.docker.internal:3306` where
   `host.docker.internal` translates to the host ip address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).

   If you are running MySQL on remote, please be aware that MySQL only allows connections from localhost
   by default. Fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-mysql).

1. Open a Web browser and direct to `localhost:8888` and input the token. Then you
can create notebooks. In a cell, you should be able to type

   ```
   %%sqlflow
   select 1
   ```

1. Explore more examples at [example.ipynb](/example/jupyter/example.ipynb)

## Demo 2: Command Line Prompt

Start a Docker container that runs SQLFlow command line prompt. If you are using
Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`.

```
docker run -it --rm --net=host sqlflow/sqlflow:latest demo \
--db_user root --db_password root --db_address host.docker.internal:3306
```

You should be able to see the following prompt.

```
sqlflow>
```

### Example

- Select data
```sql
sqlflow> select * from iris.train limit 2;
-----------------------------
+--------------+-------------+--------------+-------------+-------+
| SEPAL LENGTH | SEPAL WIDTH | PETAL LENGTH | PETAL WIDTH | CLASS |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
|            5 |         2.3 |          3.3 |           1 |     1 |
+--------------+-------------+--------------+-------------+-------+
```
- Train a Tensorflow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier)
```sql
sqlflow> SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
-----------------------------
...
Training set accuracy: 0.96721
Done training
```
- Prediction using a trained model
```sql
sqlflow> SELECT *
FROM iris.test
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
```
- Checkout prediction result
```sql
sqlflow> select * from iris.predict limit 10;
```