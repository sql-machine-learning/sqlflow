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

# Wait until hive test server is ready, port 8899
# is a status port indicates the hive server container
# is ready, see .travis.yml for the details
while true; do
  curl http://localhost:8899 2>/dev/null
  if [ $? -eq 0 ]; then
    break
  else
    echo "still waiting, hive server is not ready..."
    sleep 5
  fi
done

set -e

hdfs dfs -rm -r -f hdfs://localhost:8020/sqlflow
hdfs dfs -mkdir -p hdfs://localhost:8020/sqlflow
export SQLFLOW_HIVE_LOCATION_ROOT_PATH=/sqlflow
export SQLFLOW_TEST_DB=hive
# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

go generate ./...
go install ./...

# -p 1 is necessary since tests in different packages are sharing the same database
# ref: https://stackoverflow.com/a/23840896
SQLFLOW_log_level=debug go test -p 1 -v ./...
SQLFLOW_codegen=ir SQLFLOW_log_level=debug go test -p 1 -v ./cmd/... -run TestEnd2EndHiveIR

python -m unittest discover -v python "db_test.py"
