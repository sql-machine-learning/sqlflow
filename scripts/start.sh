#!/bin/bash
# Copyright 2019 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SQLFLOW_MYSQL_HOST=${SQLFLOW_MYSQL_HOST:-127.0.0.1}
SQLFLOW_MYSQL_PORT=${SQLFLOW_MYSQL_PORT:-3306}

function setup_mysql() {
    # Start mysqld
    sed -i "s/.*bind-address.*/bind-address = ${SQLFLOW_MYSQL_HOST}/" /etc/mysql/mysql.conf.d/mysqld.cnf
    service mysql start
    sleep 1
    until mysql -u root -proot --host ${SQLFLOW_MYSQL_HOST} --port ${SQLFLOW_MYSQL_PORT} -e ";" ; do
        sleep 1
        read -p "Can't connect, retrying..."
    done
    # FIXME(typhoonzero): should let docker-entrypoint.sh do this work
    for f in /docker-entrypoint-initdb.d/*; do
        cat $f | mysql -uroot -proot --host ${SQLFLOW_MYSQL_HOST} --port ${SQLFLOW_MYSQL_PORT}
    done
    # Grant all privileges to any remote hosts so that the sqlserver can be scaled into more than one replicas.
    mysql -uroot -proot -e "GRANT ALL PRIVILEGES ON *.* TO 'root'@'' IDENTIFIED BY 'root' WITH GRANT OPTION;"
}

function setup_sqlflow() {
  DS="mysql://root:root@tcp(${SQLFLOW_MYSQL_HOST}:${SQLFLOW_MYSQL_PORT})/?maxAllowedPacket=0"
  echo "Connect to the datasource ${DS}"
  # Start sqlflowserver
  sqlflowserver --datasource=${DS} &
  # Start jupyter notebook
  SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''
}

function print_usage() {
  echo "Usage: /bin/bash start.sh [OPTION]\n"
  echo "\tmysql: setup the mysql server with the example dataset initialized."
  echo "\tsqlflow: setup the sqlflow server and the jupyter notebook which port is 8888"
  echo "\tall(default): setup mysql server and sqlflow server in one container."
}

function main() {
  ARG=${1:-all}
  case $ARG in 
    mysql)
      setup_mysql
      sleep infinity
      ;;
    sqlflow)
      setup_sqlflow
      ;;
    all)
      echo "setup all-in-one"
      setup_mysql
      setup_sqlflow 
      ;;
    *)
      print_usage
      ;;
  esac
}

main $@
