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

set -e

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

export SQLFLOW_TEST_DB=hive

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_log_level=debug go test -v ./...

python -m unittest discover -v sql/python "db_test.py"
