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

export SQLFLOW_TEST_DB=maxcompute
export MAXCOMPUTE_ENDPOINT="service.cn.maxcompute.aliyun.com/api?curr_project=gomaxcompute_driver_w7u"
if [ "$MAXCOMPUTE_AK" = "" ] || [ "$MAXCOMPUTE_SK" == "" ]; then
  echo "skip maxcompute test because the env MAXCOMPUTE_AK or MAXCOMPUTE_SK is empty"
  exit 0
fi
# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/github.com/sql-machine-learning/sqlflow/sql/python

go generate ./...
go get -v -t ./...
go install ./...

# -p 1 is necessary since tests in different packages are sharing the same database
# ref: https://stackoverflow.com/a/23840896
SQLFLOW_log_level=debug go test -p 1 -v ./...
