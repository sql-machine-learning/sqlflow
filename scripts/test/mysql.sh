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

changed_fileext=$(git diff --name-only HEAD..origin/develop --|awk -F. '{print $NF}'|uniq)
if [[ "$changed_fileext" == "md" ]]; then
    echo "Only Markdown files changed.  No need to run unit tests."
    exit 0
fi

# Wait for MySQL server to initialize, the the sqlflow/sqlflow:mysql will
# start an HTTP server at 8890
while true; do
    if [ -f mysql-inited ]; then
        break
    else
        echo "still waiting, MySQL server is not ready..."
        sleep 1
    fi
done

export SQLFLOW_TEST_DB=mysql

python -c "import sqlflow_models"
python -c "import runtime.db"

go generate ./...
go install ./...
gotest -p 1 -covermode=count -coverprofile=coverage.txt -timeout 900s  -v ./...
python -m unittest discover -v python "*_test.py"
