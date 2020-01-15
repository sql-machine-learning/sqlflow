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

function sleep_until_mysql_is_ready() {
  until mysql -u root -proot --host 127.0.0.1 --port 3306 -e ";" ; do
    sleep 1
    read -p "Can't connect, retrying..."
  done
}


function populate_example_dataset() {
  sleep_until_mysql_is_ready
  # FIXME(typhoonzero): should let docker-entrypoint.sh do this work
  for f in /docker-entrypoint-initdb.d/*; do
    cat $f | mysql -uroot -proot --host 127.0.0.1  --port 3306
  done
}

set -e

service mysql start
sleep 1
populate_example_dataset

go generate ./...
go install ./...

DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"

# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

sqlflowserver &
# e2e test for standard SQL
SQLFLOW_DATASOURCE=${DATASOURCE} SQLFLOW_SERVER=localhost:50051 ipython python/test_magic.py
# TODO(yi): Re-enable the end-to-end test of Ant XGBoost after accelerating Travis CI.
# SQLFLOW_SERVER=localhost:50051 ipython sql/python/test_magic_ant_xgboost.py
# TODO(terrytangyuan): Enable this when ElasticDL is open sourced
# e2e test for ElasticDL SQL
# export SQLFLOW_submitter=elasticdl
# SQLFLOW_SERVER=localhost:50051 ipython sql/python/test_magic_elasticdl.py
