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
export PYTHONPATH=$GOPATH/src/github.com/sql-machine-learning/sqlflow/sql/python

python -c "import sqlflow_models"
python -c "import sqlflow_submitter.db"
#python -c "from tensorflow import keras; keras.datasets.imdb.load_data(num_words=10000)"

go generate ./...
go get -v -t ./...
go install ./...

# -p 1 is necessary since tests in different packages are sharing the same database
# ref: https://stackoverflow.com/a/23840896
SQLFLOW_log_level=debug go test -v -p 1 ./...  -covermode=count -coverprofile=coverage.out

python -m unittest discover -v sql/python "*_test.py"
