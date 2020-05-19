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
    until mysql -u root -proot \
                --host "$SQLFLOW_MYSQL_HOST" \
                --port "$SQLFLOW_MYSQL_PORT" \
                -e ";" ; do
        sleep 1
        read -r -p "Can't connect, retrying..."
    done
}

function start_mysql() {
  # Start mysqld
    sed -i "s/.*bind-address.*/bind-address = ${SQLFLOW_MYSQL_HOST}/" \
        /etc/mysql/mysql.conf.d/mysqld.cnf
  service mysql start
  sleep 1
}

function setup_mysql() {
  start_mysql
  populate_example_dataset
  # Grant all privileges to all the remote hosts so that the sqlflow
  # server can be scaled to more than one replicas.
  #
  # NOTE: should notice this authorization on the production
  # environment, it's not safe.
  mysql -uroot -proot \
        -e "GRANT ALL PRIVILEGES ON *.* TO 'root'@'' IDENTIFIED BY 'root' WITH GRANT OPTION;"
}

function populate_example_dataset() {
  sleep_until_mysql_is_ready
  # FIXME(typhoonzero): should let docker-entrypoint.sh do this work
  for f in /docker-entrypoint-initdb.d/*; do
      mysql -uroot -proot \
            --host "$SQLFLOW_MYSQL_HOST" --port "$SQLFLOW_MYSQL_PORT" \
            < "$f"
  done
}

function setup_sqlflow_server() {
  sleep_until_mysql_is_ready

  # Start sqlflowserver
  sqlflowserver
}

function setup_sqlflow_notebook() {
    cd "$SQLFLOW_NOTEBOOK_DIR" ||
        ( echo "Cannot cd to $SQLFLOW_NOTEBOOK_DIR"; exit 1 )
  DS="mysql://root:root@tcp(${SQLFLOW_MYSQL_HOST}:${SQLFLOW_MYSQL_PORT})/?maxAllowedPacket=0"
  echo "Connect to the datasource $DS ..."
  SQLFLOW_DATASOURCE="$DS" SQLFLOW_SERVER="localhost:50051" \
                    jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root \
                    --NotebookApp.token=''
  cd ..
}

function print_usage() {
  echo "Usage: start.sh [OPTION]"
  echo "  populate-example-dataset-mysql: populate an existing mysql instance"
  echo "    with the example dataset."
  echo "  mysql: setup the mysql server with the example dataset initialized."
  echo "  sqlflow_server: setup the sqlflow gRPC server."
  echo "  sqlflow_notebook: setup the Jupyter Notebook server."
  echo "  all(default): setup a MySQL server instance, a sqlflow gRPC server"
  echo "    and a Jupyter Notebook server sequentially."
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
    sqlflow-server-with-dataset)
      setup_mysql
      setup_sqlflow_server
      ;;
    all)
      echo "setup all-in-one"
      setup_mysql
      setup_sqlflow_server &
      setup_sqlflow_notebook
      ;;
    *)
      print_usage
      ;;
  esac
}

main "$@"
