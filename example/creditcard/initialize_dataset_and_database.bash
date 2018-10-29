#!/bin/bash
################################################################################

if [[ $(which docker) == "" ]]; then
    echo "Cannot find Docker.  "
    echo "Please install it referring https://docs.docker.com/install"
    exit
fi

docker version > /dev/null
if [[ $? != "0" ]]; then
    echo "The command docker version doesn't work as expected."
    echo "Please check the reason and retry."
    exit
fi

if [[ $(which kaggle) == "" ]]; then
    echo "Cannot find the command line tool kaggle.  Try to install it ...";
    pip install kaggle --upgrade
    if [[ $? != "0" ]]; then
	echo "Failed to install kaggle."
	echo "Please check https://github.com/Kaggle/kaggle-api"
	exit
    fi
fi

if [[ ! -f $HOME/.kaggle/kaggle.json ]]; then
    echo "Please sign up and get your API credentials from kaggle.com"
    echo "Please check https://github.com/Kaggle/kaggle-api"
    exit
fi

export SQLFLOW_DB_DIR=$HOME/.sqlflow/example/db/creditcard
export SQLFLOW_DATABASE=creditcard

echo "Cleanup legacy resource ..."
docker stop mysql-creditcard
docker rm mysql-creditcard
rm -rf $SQLFLOW_DB_DIR
mkdir -p $SQLFLOW_DB_DIR

echo "Starting the MySQL server in a Docker container ..."
docker run --rm \
       -v $SQLFLOW_DB_DIR:/var/lib/mysql \
       --name mysql-creditcard \
       -e MYSQL_ROOT_PASSWORD=root \
       -e MYSQL_ROOT_HOST='%' \
       -p 3306:3306 \
       -d \
       mysql/mysql-server:8.0 \
       mysqld --secure-file-priv=""

echo "Downloading the dataset ..."
kaggle datasets download -p /tmp --unzip \
       -d mlg-ulb/creditcardfraud

if [[ ! -S $SQLFLOW_DB_DIR/mysql.sock ]]; then
    echo "Cannot find " $SQLFLOW_DB_DIR/mysql.sock
    echo "Waiting for 10 more seconds ..."
    sleep 10
    if [[ ! -S $SQLFLOW_DB_DIR/mysql.sock ]]; then
	echo "Seems that something went wrong."
	exit
    fi
fi

echo "Creating the database and table creditcard.creditcard ..."
docker run --rm -it \
       -v $SQLFLOW_DB_DIR:/var/lib/mysql \
       mysql/mysql-server:8.0 \
       mysql --user root --password=root \
       -e 'CREATE DATABASE creditcard;
USE creditcard;
CREATE TABLE IF NOT EXISTS creditcard (
    time INT,
	v1 float,
	v2 float,
	v3 float,
	v4 float,
	v5 float,
	v6 float,
	v7 float,
	v8 float,
	v9 float,
	v10 float,
	v11 float,
	v12 float,
	v13 float,
	v14 float,
	v15 float,
	v16 float,
	v17 float,
	v18 float,
	v19 float,
	v20 float,
	v21 float,
	v22 float,
	v23 float,
	v24 float,
	v25 float,
	v26 float,
	v27 float,
	v28 float,
	amount float,
	class varchar(255));'

if [[ $? != "0" ]]; then
    echo "Failed to create table"
    exit
fi

if [[ ! -f /tmp/creditcard.csv ]]; then
    echo "Failed to download the dataset"
    exit
fi

echo "Importing the CSV file ..."
mv /tmp/creditcard.csv $SQLFLOW_DB_DIR/creditcard
docker run --rm -it \
       -v $SQLFLOW_DB_DIR:/var/lib/mysql \
       mysql:8.0 \
       mysqlimport \
       --ignore-lines=1  \
       --fields-terminated-by=,  \
       --socket /var/lib/mysql/mysql.sock \
       -u root --password=root \
       creditcard creditcard.csv 
