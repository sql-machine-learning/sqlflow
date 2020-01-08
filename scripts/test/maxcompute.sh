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

if [[ $(git diff --name-only HEAD..develop|awk -F. '{print $NF}'|uniq) == md ]]; then
  exit
fi

export SQLFLOW_TEST_DB=maxcompute
export SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT="service.cn.maxcompute.aliyun.com/api?curr_project=gomaxcompute_driver_w7u&scheme=https"
export SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT="gomaxcompute_driver_w7u"
export SQLFLOW_TEST_DB_MAXCOMPUTE_AK=$MAXCOMPUTE_AK # TODO(wangkuiyi): Remove after rename env variable in Travis CI.
export SQLFLOW_TEST_DB_MAXCOMPUTE_SK=$MAXCOMPUTE_SK
if [ "$SQLFLOW_TEST_DB_MAXCOMPUTE_AK" = "" ] || [ "$SQLFLOW_TEST_DB_MAXCOMPUTE_SK" == "" ]; then
  echo "skip maxcompute test because the env SQLFLOW_TEST_DB_MAXCOMPUTE_AK or SQLFLOW_TEST_DB_MAXCOMPUTE_SK is empty"
  exit 0
fi
# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

go generate ./...
go install ./...

# -p 1 is necessary since tests in different packages are sharing the same database
# ref: https://stackoverflow.com/a/23840896
# TODO(Yancey1989): enable all the unit test for the maxcompute
SQLFLOW_log_level=debug gotest -p 1 -v ./cmd/... -run TestEnd2EndMaxCompute

# TODO(shendiaomo): fix CI after the PAI service initiated in the MaxCompute project
# export SQLFLOW_submitter=pai
# SQLFLOW_log_level=debug gotest -p 1 -v ./cmd/... -run TestEnd2EndMaxCompute

function test_end2end_elasticdl() {
  export SQLFLOW_submitter=elasticdl
  cd /elasticdl
  # Build base images for ElasticDL jobs
  docker build -t elasticdl:dev -f elasticdl/docker/Dockerfile.dev .
  docker build -t elasticdl:ci -f elasticdl/docker/Dockerfile.ci .
  # Set up necessary RBAC roles for k8s cluster
  kubectl apply -f elasticdl/manifests/examples/elasticdl-rbac.yaml
  cd -
  SQLFLOW_log_level=debug gotest -p 1 -v ./cmd/... -run TestEnd2EndMaxComputeElasticDL

  cd /elasticdl
  bash scripts/validate_job_status.sh odps
}
# uncomment the below line to enable end-to-end test for ElasticDL.
# test_end2end_elasticdl
