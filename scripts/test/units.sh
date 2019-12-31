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

service mysql start

export SQLFLOW_TEST_DB=mysql
# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

python -c "import sqlflow_models"
python -c "import sqlflow_submitter.db"

go generate ./...

go install ./...

# -p 1 is necessary since tests in different packages are sharing the same database
# ref: https://stackoverflow.com/a/23840896
# set test timeout to 900s since travis CI may be slow to run the case TestParse
SQLFLOW_log_level=debug gotest -v -p 1 -timeout 900s ./...  -covermode=count -coverprofile=coverage.out

python -m unittest discover -v python "*_test.py"
