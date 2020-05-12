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

go env

echo "Install protoc ..."
PROTOC_SITE="https://github.com/protocolbuffers/protobuf/releases/download"
curl -sL $PROTOC_SITE"/v3.7.1/protoc-3.7.1-osx-x86_64.zip" -o p.zip
unzip -qq p.zip -d /usr/local
rm p.zip

echo "Install goyacc and protoc-gen-go ..."
go get \
   github.com/golang/protobuf/protoc-gen-go@v1.3.3 \
   golang.org/x/tools/cmd/goyacc

echo "Build cmd/sqlflow into /tmp ..."
cd $TRAVIS_BUILD_DIR
go generate ./...
GOBIN=/tmp go install ./cmd/sqlflow

echo "Publish /tmp/sqlflow to the Wiki repo ..."
cd /tmp
REPO=github.com/sql-machine-learning/sqlflow.wiki.git
git clone https://$REPO
cd sqlflow.wiki
mv /tmp/sqlflow .
git add sqlflow
git commit -am 'Add/update sqlflow'
git push --all https://$GITHUB_USERNAME:$GITHUB_PASSWORD@$REPO
