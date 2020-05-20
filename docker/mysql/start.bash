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

echo "Start mysqld ..."
# Important to make mysqld bind to 0.0.0.0 -- all IPs.  I explained
# the reason in https://stackoverflow.com/a/61887788/724872.
MYSQL_HOST=${MYSQL_HOST:-0.0.0.0}
sed -i "s/.*bind-address.*/bind-address = $MYSQL_HOST/" \
    /etc/mysql/mysql.conf.d/mysqld.cnf
service mysql start


echo "Sleep until MySQL server is ready ..."
# shellcheck disable=SC2153
until mysql -u root -proot \
            --host "$MYSQL_HOST" \
            --port "$MYSQL_PORT" \
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


# FIXME(typhoonzero): should let docker-entrypoint.sh do this work
for f in /datasets/*; do
    echo "Populate datasets $f ..."
    mysql -uroot -proot \
          --host "$SQLFLOW_MYSQL_HOST" --port "$SQLFLOW_MYSQL_PORT" \
          < "$f"
done


# If we run the contaienr with -v host_dir:/work, then the following
# command would create host_dir/mysql-inited file.  A bash script (on
# the host or a container) would be able to wait the creation of this
# file using the trick https://unix.stackexchange.com/a/185370/325629.
touch /work/mysql-inited

sleep infinity
