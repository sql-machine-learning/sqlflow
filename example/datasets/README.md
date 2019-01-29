# The MySQL Server Container for Testing

This directory contains a Dockerfile that builds a Docker image derived the MySQL Server 8.0 image, and includes SQL programs that popularize the following datasets:

1. [Churn from Kaggle](https://www.kaggle.com/blastchar/telco-customer-churn)
1. [Irises classfication from TensorFlow](https://www.tensorflow.org/guide/premade_estimators#classifying_irises_an_overview)

We can run a Docker container of it for unit testing.

## Build

```bash
docker build -t sqlflowtest .
```

## Run

```bash
docker run --rm -d --name sqlflowtest \
   -p 3306:3306 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST=% \
   sqlflowtest
```

## Popularize Datasets

We need to manually popularize the databases and tables:

```bash
docker exec -it sqlflowtest bash
```

To popularize the Churn dataset into `churn.churn`:

```bash
cat /popularize_churn.sql | mysql -uroot -proot
```

To popularize the Irises dataset into `iris.iris`:

```bash
cat /popularize_iris.sql | mysql -uroot -proot
```

## Query

In the container, run

```bash
echo "select count(*) from churn.churn;" | mysql -uroot -proot
```

should print the number of rows as the following

```
count(*)
92
```

## Trouble shooting:

1. It usually takes about 15 seconds to bring up the MySQL Server. If you try to connect it
before that, you may see the following error

```
ERROR 1045 (28000): Access denied for user 'root'@'localhost' (using password: YES)
```
