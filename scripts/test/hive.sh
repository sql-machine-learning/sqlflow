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

# Wait until hive test server is ready, port 8899
# is a status port indicates the hive server container
# is ready, see .travis.yml for the details

changed_fileext=$(git diff --name-only HEAD..origin/develop --|awk -F. '{print $NF}'|uniq)
if [[ "$changed_fileext" == "md" ]]; then
    echo "Only Markdown files changed.  No need to run unit tests."
    exit 0
fi

# Wait for Hive server to start on port 8899.
while true; do
    if curl -s http://localhost:8899 > /dev/null 2>&1; then
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
export SQLFLOW_TEST_NAMENODE_ADDR="127.0.0.1:8020"

export SQLFLOW_TEST_DB=hive
export SQLFLOW_USE_EXPERIMENTAL_CODEGEN=true

# NOTE: we have already installed runtime under Python installation
# path using latest develop branch, but when testing on CI, we need to use the
# code in the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python:$PYTHONPATH

go generate ./...
go install ./...
gotest -p 1 -covermode=count -coverprofile=coverage.txt -timeout 1800s  -v ./...
coverage run -m unittest discover -v python "db_test.py"
coverage run -m unittest discover -v python "hive_test.py"
