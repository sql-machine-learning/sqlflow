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



# Start mysqld
service mysql start
sleep 1
until mysql -u root -proot  -e ";" ; do
    sleep 1
    read -p "Can't connect, retrying..."
done
# FIXME(typhoonzero): should let docker-entrypoint.sh do this work
for f in /docker-entrypoint-initdb.d/*; do
    cat $f | mysql -uroot -proot
done
# Start sqlflowserver
sqlflowserver --datasource='mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0' &
# Start jupyter notebook
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root
