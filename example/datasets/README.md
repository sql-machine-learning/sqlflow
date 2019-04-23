# Building and running MySQL server 8.0

This directory contains a Dockerfile that builds a Docker image derived the MySQL Server 8.0 image. It includes SQL programs that popularize the following datasets:

1. [Churn from Kaggle](https://www.kaggle.com/blastchar/telco-customer-churn)
1. [Irises classfication from TensorFlow](https://www.tensorflow.org/guide/premade_estimators#classifying_irises_an_overview)

We can run a Docker container of it for unit testing.

## Build SQLFlow image

```bash
cd example/datasets
docker build -t sqlflow:data .
```

## Run SQLFlow docker image

```bash
docker run --rm -d --name sqlflowdata \
   -p 3306:3306 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST=% \
   sqlflow:data
```

## Popularize Datasets to MySQL

Popularize the databases and tables with commands below:

```bash
docker exec -it sqlflowdata bash
```

To popularize the Churn dataset into `churn`:

```bash
cat /popularize_churn.sql | mysql -uroot -proot
```

To popularize the Irises dataset into `iris`:

```bash
cat /popularize_iris.sql | mysql -uroot -proot
```

To prepare database for storing machine learning models:

```bash
echo "CREATE DATABASE IF NOT EXISTS sqlflow_models;" | mysql -uroot -proot
```

## Try your first Query in the container

In the container, run below command to test if tables exist. 

```bash
echo "select count(*) from churn.test;" | mysql -uroot -proot
```

It should print the number of rows as the following:

```
count(*)
10
```
## Trouble Shooting

1. It usually takes about 15 seconds to bring up the MySQL Server. If you try to connect it
before that, you may see the following error

```
ERROR 1045 (28000): Access denied for user 'root'@'localhost' (using password: YES)
```

After commands executes successfully and data are popularized, go back to Installation page. 