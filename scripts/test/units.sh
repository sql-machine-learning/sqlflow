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

changed_fileext=$(git diff --name-only HEAD..develop|awk -F. '{print $NF}'|uniq)
if [[ "$changed_fileext" == "md" ]]; then
    echo "Only Markdown files changed.  No need to run unit tests."
    exit 0
fi

# Wait for the creation of file /work/mysql-inited.  The entrypoint
# of sqlflow:mysql should create this file on a bind mount of the host
# filesystem.  So, the container running this script should also bind
# mount the same host directory to /work.
while read -r i; do if [ "$i" = "mysql-inited" ]; then break; fi; done \
    < <(inotifywait  -e create,open --format '%f' --quiet /work --monitor)

export SQLFLOW_TEST_DB=mysql

python -c "import sqlflow_models"
python -c "import sqlflow_submitter.db"

go generate ./...
go install ./...

# See https://github.com/codecov/example-go for merging multiple coverage output
# files into one.
gotest -covermode=count -coverprofile=profile.out -timeout 900s  -v ./...
if [ -f profile.out ]; then
    cat profile.out >> coverage.txt
    rm profile.out
fi

python -m unittest discover -v python "*_test.py"
