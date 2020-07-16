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

export SQLFLOW_TEST_DB=maxcompute
export SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT="service.cn.maxcompute.aliyun.com/api?curr_project=gomaxcompute_driver_w7u&scheme=https"
export SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT="gomaxcompute_driver_w7u"
if [ "$SQLFLOW_TEST_DB_MAXCOMPUTE_AK" = "" ] || [ "$SQLFLOW_TEST_DB_MAXCOMPUTE_SK" == "" ]; then
    echo "Skip MaxCompute tests because SQLFLOW_TEST_DB_MAXCOMPUTE_AK or SQLFLOW_TEST_DB_MAXCOMPUTE_SK is empty"
    exit 0
fi

# NOTE: we have already installed runtime under python installation
# path using latest develop branch, but when testing on CI, we need to use the
# code in the current pull request.
export PYTHONPATH=/work/python:$GOPATH/src/sqlflow.org/sqlflow/python:$PYTHONPATH

go generate ./...
go install ./...

# TODO(Yancey1989): enable all the unit test for MaxCompute
#
# Refer to https://github.com/codecov/example-go for merging coverage from
# multiple runs of tests.
gotest -p 1 -covermode=count -coverprofile=profile.out -timeout 1800s -v ./go/cmd/sqlflowserver \
       -run TestEnd2EndMaxCompute
if [ -f profile.out ]; then
    cat profile.out > coverage.txt
    rm profile.out
fi
gotest -p 1 -covermode=count -coverprofile=profile.out -v ./go/sqlfs/...
if [ -f profile.out ]; then
    cat profile.out >> coverage.txt
    rm profile.out
fi

python -m unittest discover -v python "db_test.py"

# TODO(shendiaomo): fix CI after the PAI service initiated in the MaxCompute
# project.
# export SQLFLOW_submitter=pai
# gotest -p 1 -covermode=count -coverprofile=profile.out -p 1 -v ./cmd/... -run TestEnd2EndMaxCompute

# Uncomment the below line to enable end-to-end test for ElasticDL.
# export SQLFLOW_submitter=elasticdl
# cd /elasticdl
# # Build base images for ElasticDL jobs
# docker build -t elasticdl:dev -f elasticdl/docker/Dockerfile.dev .
# docker build -t elasticdl:ci -f elasticdl/docker/Dockerfile.ci .
# # Set up necessary RBAC roles for k8s cluster
# kubectl apply -f elasticdl/manifests/examples/elasticdl-rbac.yaml
# cd -
# gotest -p 1 -v ./cmd/... -run TestEnd2EndMaxComputeElasticDL
# cd /elasticdl
# bash scripts/validate_job_status.sh odps
