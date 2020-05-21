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

echo "Start SQLFlow server ..."
sqlflowserver &

# Wait for the creation of file /work/mysql-inited.  The entrypoint
# of sqlflow:mysql should create this file on a bind mount of the host
# filesystem.  So, the container running this script should also bind
# mount the same host directory to /work.
while read i; do if [ "$i" = "mysql-inited" ]; then break; fi; done \
    < <(inotifywait  -e create,open --format '%f' --quiet /work --monitor)

echo "Setup Jupyter notebook connecting to $DS ..."
# The following data source URL implies that the MySQL server runs in
# a container, and the container running this script must have the
# option --net=container:mysql_server_container, so this script can
# access the MySQL server running in another container as it runs in
# the same container.
SQLFLOW_DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0" \
SQLFLOW_SERVER="localhost:50051" \
  jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''
