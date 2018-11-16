# scheduler

## Overview

This package implements a job scheduler in Python, which is used to parse the `json` job description and schedules the ML job using TF. The whole procedure includes

1. Parsing the `json` job description.
1. Retrieving the data from MySQL via [MySQL Connector Python API](https://dev.mysql.com/downloads/connector/python/). Optionally, retrieving the model from MySQL.
1. Training the model or predicts using the trained model by calling the user specified TensorFlow estimator.
1. Writing the trained model or prediction results into a table.

## Demo

Build scheduler docker image

```
docker build -t sqlflow ..
```

Start a MySQL Server as documented [here](https://github.com/wangkuiyi/sqlflow/blob/develop/doc/mysql-setup.md)

```bash
docker run --rm \
   -v /tmp/test1:/var/lib/mysql \
   --name mysql01 \
   -e MYSQL_ROOT_PASSWORD=root \
   -e MYSQL_ROOT_HOST='%' \
   -p 3306:3306 \
   -d \
   mysql/mysql-server:8.0
```


cat sample.json | python scheduler.py
