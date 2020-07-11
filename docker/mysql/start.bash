#!/bin/sh

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


echo "Init mysqld if needed ..."
if [ -d "/docker-entrypoint-initdb.d" ]; then
    echo "Skip"
else
    mkdir -p /var/run/mysqld
    mkdir -p /var/lib/mysql
    chown mysql:mysql /var/run/mysqld
    chown mysql:mysql /var/lib/mysql
    mkdir -p /docker-entrypoint-initdb.d

    mysql_install_db --user=mysql --datadir=/var/lib/mysql >dev/null
    mysqld --user=mysql --bootstrap --verbose=0 \
        --skip-name-resolve --skip-networking=0 >/dev/null <<EOF
FLUSH PRIVILEGES;
DELETE FROM mysql.user;
GRANT ALL ON *.* TO 'root'@'%' identified by 'root' WITH GRANT OPTION;
DROP DATABASE IF EXISTS test;
FLUSH PRIVILEGES;
EOF
fi


echo "Start mysqld ..."
# Important to make mysqld bind to 0.0.0.0 -- all IPs.  I explained
# the reason in https://stackoverflow.com/a/61887788/724872.
MYSQL_HOST=${MYSQL_HOST:-0.0.0.0}
MYSQL_PORT=${MYSQL_PORT:-3306}

nohup mysqld --user=mysql --console \
    --skip-name-resolve --skip-networking=0 >/dev/null 2>&1 &
sleep 2


echo "Sleep until MySQL server is ready ..."
# shellcheck disable=SC2153
until mysql -u root -proot \
            --host "$MYSQL_HOST" \
            --port "$MYSQL_PORT" \
            -e ";" ; do
    sleep 1
    echo "Can't connect, retrying..."
done


echo "Populate datasets ..."
for f in /datasets/*; do
    echo "$f"
    mysql -uroot -proot < "$f"
done
echo "Done."

# If we run the contaienr with -v host_dir:/work, then the following
# command would create host_dir/mysql-inited file.  A bash script (on
# the host or a container) would be able to wait the creation of this
# file using the trick https://unix.stackexchange.com/a/185370/325629.
mkdir -p /work && touch /work/mysql-inited && chmod 777 /work/mysql-inited


# c.f. https://stackoverflow.com/questions/2935183/bash-infinite-sleep-infinite-blocking for BusyBox
echo "Serving ..."
while true; 
    do sleep 1d;
done
