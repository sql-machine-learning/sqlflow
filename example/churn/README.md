# The MySQL Server Container for Testing

This image contains MySQL Server and a [small dataset](https://www.kaggle.com/blastchar/telco-customer-churn). We can run a container of it for unit testing -- unit tests could connect to the MySQL Server service running locally.

## Build

```bash
docker built -t sqlflowtest .
```

## Run

```bash
docker run --rm -d --name sqlflowtest \
   -p 3306:3306 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST='%' \
   sqlflowtest
```

Manually popularize the database and table:

```bash
docker exec -it sqlflowtest bash
```

In the container, run

```bash
cat /popularize_table.sql | mysql -uroot -proot
```

## Check

In the container, run

```bash
echo "select count(*) from churn.churn;" | mysql -uroot -proot
```

should print the number of rows as the following

```
count(*)
7032
```

