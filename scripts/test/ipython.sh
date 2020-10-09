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

DS="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"

go generate ./...
go install ./...

# NOTE: we have already installed runtime under python
# installation path using latest develop branch, but when testing on
# CI, we need to use the code in the current pull request.
export PYTHONPATH=$GOPATH/src/sqlflow.org/sqlflow/python

killall sqlflowserver || true
sqlflowserver &
sleep 10

# Setup sqlflow magic for testing.
# NOTE(typhoonzero): In the sqlflow/sqlflow:jupyter Docker image, this is
# installed by install-jupyter.sh, you should update both places.
pip install sqlflow==0.14.0
IPYTHON_STARTUP="$HOME/.ipython/profile_default/startup/"
mkdir -p "$IPYTHON_STARTUP"
if [ ! -e "$IPYTHON_STARTUP"/00-first.py ]; then
{ echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")';
  echo 'get_ipython().magic(u"%reload_ext autoreload")';
  echo 'get_ipython().magic(u"%autoreload 2")'; } \
    >> "$IPYTHON_STARTUP"/00-first.py
fi

SQLFLOW_DATASOURCE="$DS" \
SQLFLOW_SERVER="localhost:50051" \
  ipython python/test_magic.py

# TODO(yi): Re-enable the end-to-end test of Ant XGBoost after accelerating Travis CI.
# SQLFLOW_SERVER=localhost:50051 ipython sql/python/test_magic_ant_xgboost.py
# TODO(terrytangyuan): Enable this when ElasticDL is open sourced
# e2e test for ElasticDL SQL
# export SQLFLOW_submitter=elasticdl
# SQLFLOW_SERVER=localhost:50051 ipython sql/python/test_magic_elasticdl.py
