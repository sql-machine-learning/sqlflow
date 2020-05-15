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

set -x # DEBUG
set -e

# For more informaiton about deployment with Travis CI, please refer
# to the file header of deploy_docker.sh

if [[ "$TRAVIS_PULL_REQUEST" != "false" ]]; then
    echo "Skip deployment on pull request"
    exit 0
fi

# Figure out the tag to push sqlflow:ci.
if [[ "$TRAVIS_BRANCH" == "develop" ]]; then
    if [[ "$TRAVIS_EVENT_TYPE" == "cron" ]]; then
        RELEASE_TAG="nightly"
    else
        RELEASE_TAG="latest"
    fi
elif [[ "$TRAVIS_TAG" != "" ]]; then
    RELEASE_TAG="$TRAVIS_TAG"
else
    echo "Cannot figure out Docker image tag."
    exit 1
fi


echo "Verify Go is installed ..."
go env


echo "Install download tools ..."
case "$TRAVIS_OS_NAME" in
    linux)
        sudo apt-get -qq update > /dev/null
        sudo apt-get -qq install -y axel unzip > /dev/null
        ;;
    windows) choco install axel ;;
    osx) brew install axel ;;
esac


echo "Install protoc ..."
case "$TRAVIS_OS_NAME" in
    linux)
        # The following code snippet comes from docker/dev/install.sh
        echo "Install protoc ..."
        PROTOC_SITE="https://github.com/protocolbuffers/protobuf/releases"
        axel --quiet --output "p.zip" \
             $PROTOC_SITE"/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip"
        sudo unzip -qq p.zip -d /usr/local
        rm p.zip
        ;;
    windows) choco install protoc ;;
esac
protoc --version


echo "Install goyacc and protoc-gen-go ..."
go get \
   github.com/golang/protobuf/protoc-gen-go@v1.3.3 \
   golang.org/x/tools/cmd/goyacc
export PATH=$GOPATH/bin:$PATH


echo "Build cmd/sqlflow into /tmp ..."
cd "$TRAVIS_BUILD_DIR"
go generate ./...
GOBIN=/tmp go install ./cmd/sqlflow


echo "Install Qiniu client for $TRAVIS_OS_NAME ..."
case "$TRAVIS_OS_NAME" in
    linux) F="qshell-linux-x64-v2.4.1" ;;
    windows) F="qshell-windows-x64-v2.4.1.exe" ;;
    osx) F="qshell-darwin-x64-v2.4.1" ;;
esac
axel --quiet http://devtools.qiniu.com/$F.zip
unzip $F.zip
export PATH=$PWD:$PATH


echo "Publish /tmp/sqlflow to Qiniu Object Storage ..."
qshell account "$QINIU_AK" "$QINIU_SK" "wu"
qshell rput sqlflow-release \
       "$RELEASE_TAG/$TRAVIS_OS_NAME/sqlflow" \
       /tmp/sqlflow
