#!/bin/bash
# Copyright 2020 The SQLFlow Authors. All rights reserved.
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
SQLFLOW_NOTEBOOK_DIR=${SQLFLOW_NOTEBOOK_DIR:-/workspace}

function sleep_until_mysql_is_ready() {
  until mysql -u root -proot --host ${SQLFLOW_MYSQL_HOST} --port ${SQLFLOW_MYSQL_PORT} -e ";" ; do
    sleep 1
    read -p "Can't connect, retrying..."
  done
}

function start_mysql() {
  # Start mysqld
  sed -i "s/.*bind-address.*/bind-address = ${SQLFLOW_MYSQL_HOST}/" /etc/mysql/mysql.conf.d/mysqld.cnf
  service mysql start
  sleep 1
}

function setup_mysql() {
  start_mysql
  populate_example_dataset
  # Grant all privileges to all the remote hosts so that the sqlflow server can
  # be scaled to more than one replicas.
  # NOTE: should notice this authorization on the production environment, it's not safe.
  mysql -uroot -proot -e "GRANT ALL PRIVILEGES ON *.* TO 'root'@'' IDENTIFIED BY 'root' WITH GRANT OPTION;"
}

function populate_example_dataset() {
  sleep_until_mysql_is_ready
  # FIXME(typhoonzero): should let docker-entrypoint.sh do this work
  for f in /docker-entrypoint-initdb.d/*; do
    cat $f | mysql -uroot -proot --host ${SQLFLOW_MYSQL_HOST} --port ${SQLFLOW_MYSQL_PORT}
  done
}

function populate_example_dataset_remote() {
  # FIXME(typhoonzero): should let docker-entrypoint.sh do this work
  for f in /docker-entrypoint-initdb.d/*; do
    cat $f | repl 2>&1 >/dev/null 2>&1
  done
}

function setup_sqlflow_server() {
  sleep_until_mysql_is_ready

  # Start sqlflowserver
  sqlflowserver
}

function setup_sqlflow_notebook() {
  cd ${SQLFLOW_NOTEBOOK_DIR}
  DS="mysql://root:root@tcp(${SQLFLOW_MYSQL_HOST}:${SQLFLOW_MYSQL_PORT})/?maxAllowedPacket=0"
  echo "Connect to the datasource ${DS}"

  SQLFLOW_DATASOURCE=${DS} SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''
  cd ..
}

function print_usage() {
  echo "Usage: /bin/bash start.sh [OPTION]\n"
  echo "\tpopulate-example-dataset-mysql: populate an existing mysql instance with the example dataset."
  echo "\tmysql: setup the mysql server with the example dataset initialized."
  echo "\tsqlflow_server: setup the sqlflow gRPC server."
  echo "\tsqlflow_notebook: setup the Jupyter Notebook server."
  echo "\tall(default): setup a MySQL server instance, a sqlflow gRPC server and a Jupyter Notebook server sequentially."
  echo "\trepl: setup a MySQL server instance, a sqlflow gRPC server, a Jupyter Notebook server sequentially and enter CLI mode at last."
}

function setup_odpscmd() {
  echo access_id=$(echo $SQLFLOW_DATASOURCE|grep -oP '(?<=maxcompute://)[^:]*')>> ~/conf/odps_config.ini
  echo access_key=$(echo $SQLFLOW_DATASOURCE|grep -oP '(?<=:)[^@:/]*')>> ~/conf/odps_config.ini
  protocol=$(echo $SQLFLOW_DATASOURCE|grep -oP '(?<=scheme=)\w*')
  end_point=$(echo $SQLFLOW_DATASOURCE|grep -oP '(?<=@)[^?]*')
  echo end_point=$protocol://$end_point >> ~/conf/odps_config.ini
  echo project=$(echo $SQLFLOW_DATASOURCE|grep -oP '(?<=curr_project=)\w*')>> ~/conf/odps_config.ini
}

function main() {
  ARG=${1:-all}
  case $ARG in
    mysql)
      setup_mysql
      sleep infinity
      ;;
    populate-example-dataset-mysql)
      populate_example_dataset
      ;;
    populate-example-dataset-mysql-local)
      start_mysql
      populate_example_dataset
      ;;
    sqlflow-server)
      setup_sqlflow_server
      ;;
    sqlflow-notebook)
      setup_sqlflow_notebook
      ;;
    all)
      echo "setup all-in-one"
      setup_mysql
      setup_sqlflow_server &
      setup_sqlflow_notebook
      ;;
    repl)
      shift
      . ~/.sqlflow_env
      if [[ $SQLFLOW_DATASOURCE == "" ]]; then
        echo "No data source specified. Starting builtin MySQL"
        start_mysql
        sleep_until_mysql_is_ready
        SQLFLOW_DATASOURCE="mysql://root:root@tcp(${SQLFLOW_MYSQL_HOST}:${SQLFLOW_MYSQL_PORT})/?maxAllowedPacket=0"
	  fi
	  export SQLFLOW_DATASOURCE
      if [[ ! -d /var/lib/mysql/iris && $SQLFLOW_DATASOURCE == mysql://* ]]; then
        echo "Initializing..."
        populate_example_dataset_remote
      fi
      repl $@
      ;;
    *)
      print_usage
      ;;
  esac
}

main $@
