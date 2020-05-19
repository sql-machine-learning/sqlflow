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

set -e

SQLFLOW_MYSQL_HOST=${SQLFLOW_MYSQL_HOST:-127.0.0.1}
SQLFLOW_MYSQL_PORT=${SQLFLOW_MYSQL_PORT:-3306}


echo "Start mysqld ..."
sed -i "s/.*bind-address.*/bind-address = ${SQLFLOW_MYSQL_HOST}/" \
    /etc/mysql/mysql.conf.d/mysqld.cnf
service mysql start

echo "Sleep until MySQL server is ready ..."
until mysql -u root -proot \
            --host "$SQLFLOW_MYSQL_HOST" \
            --port "$SQLFLOW_MYSQL_PORT" \
            -e ";" ; do
    sleep 1
    read -r -p "Can't connect, retrying..."
done

# Grant all privileges to all the remote hosts so that the sqlflow
# server can be scaled to more than one replicas.
#
# NOTE: should notice this authorization on the production
# environment, it's not safe.
mysql -uroot -proot \
      -e "GRANT ALL PRIVILEGES ON *.* TO 'root'@'' IDENTIFIED BY 'root' WITH GRANT OPTION;"


echo "Populate example datasets ..."
# FIXME(typhoonzero): should let docker-entrypoint.sh do this work
for f in /datasets/*; do
    mysql -uroot -proot \
          --host "$SQLFLOW_MYSQL_HOST" --port "$SQLFLOW_MYSQL_PORT" \
          < "$f"
done
