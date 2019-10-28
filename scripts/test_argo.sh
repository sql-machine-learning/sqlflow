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

# NOTE: we have already installed sqlflow_submitter under python installation path
# using latest develop branch, but when testing on CI, we need to use the code in
# the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

go generate ./...
go get -v -t ./...
go install ./...

# End-to-end test for Argo
# Install the Argo controller and UI
kubectl create namespace argo
kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/stable/manifests/install.yaml
# Grant admin privileges to the 'default' service account in the namespace `default`, so that the service account can run workflow.
kubectl create rolebinding default-admin --clusterrole=admin --serviceaccount=default:default

SQLFLOW_log_level=debug SQLFLOW_job_runner=argo go test -p 1 -v ./pkg/server/runner/... -run TestArgoClient
