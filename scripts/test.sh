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

cat example/datasets/popularize_churn.sql | mysql -uroot -proot
cat example/datasets/popularize_iris.sql | mysql -uroot -proot
cat example/datasets/popularize_toutiao.sql | mysql -uroot -proot
cat example/datasets/create_model_db.sql | mysql -uroot -proot

export SQLFLOW_TEST_DB=mysql

python -c "import sqlflow_models"

python -c "import sqlflow_submitter.db"

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_log_level=debug go test -v ./...  -covermode=count -coverprofile=coverage.out

python -m unittest discover -v sql/python "*_test.py"
