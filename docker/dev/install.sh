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

set -ex

if [ "$FIND_FASTED_MIRROR" == "true" ]; then
    # shellcheck disable=SC1091
    source find_fastest_resources.sh
    echo "Choose the fastest APT source ..."
    choose_fastest_apt_source
    echo "Choose the fastest PIP source ..."
    choose_fastest_pip_source
fi


echo "Install apt packages ..."
DOWNLOAD_TOOLS="curl axel unzip" # We need curl to check the running of Hive.
BUILD_ESSENTIAL="build-essential git"
PYTHON_DEV="python3-dev python3-pip" # Many pip packages require Python.h
JAVA_DEV="openjdk-8-jdk maven"
SHELL_LINTER="shellcheck"
YAML_LINTER="yamllint"
OPTIMIZE_SOLVER="glpk-utils" # required solver packages of Pyomo
# shellcheck disable=SC2086
apt-get -qq update && apt-get -qq install -y --no-install-recommends \
        $DOWNLOAD_TOOLS \
        $BUILD_ESSENTIAL \
        $PYTHON_DEV \
        $JAVA_DEV \
        $SHELL_LINTER \
        $YAML_LINTER \
        $OPTIMIZE_SOLVER \
        > /dev/null
rm -rf /var/lib/apt/lists/*
apt-get -qq clean -y


echo "Make Python 3 the the default ..."
ln -s /usr/bin/python3 /usr/local/bin/python
ln -s /usr/bin/pip3 /usr/local/bin/pip

echo "Upgrade pip and setuptools creates /usr/local/bin/pip ..."
# Update setuptools because
# https://github.com/red-hat-storage/ocs-ci/pull/971/files
python -m pip install --quiet --upgrade pip setuptools six


echo "Install pip packages ..."
PRE_COMMIT="pre-commit==1.18.3"
PY_TEST="pytest==5.3.0 pytest-cov"
GRPC_PACKAGES="grpcio==1.28.1 grpcio-tools==1.28.1"
JS_LINTER=jsbeautifier
PYTHON_LINTER="yapf isort<5,>=4.2.5 pylint>=2.5.3 flake8"
WHEEL="wheel"
# shellcheck disable=SC2086
python -m pip install --quiet \
    $WHEEL \
    $PRE_COMMIT \
    $PY_TEST \
    $JS_LINTER \
    $PYTHON_LINTER \
    $GRPC_PACKAGES
rm -rf "$HOME"/.cache/pip/*


echo "Install Go compiler ..."
GO_MIRROR_0="https://studygolang.com/dl/golang/go1.13.4.linux-amd64.tar.gz"
GO_MIRROR_1="https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz"
axel --quiet --output go.tar.gz $GO_MIRROR_0 $GO_MIRROR_1
tar -C /usr/local -xzf go.tar.gz
rm go.tar.gz
export GOPATH="/root/go"
export PATH="/usr/local/go/bin:$GOPATH/bin:$PATH"


echo "Install goyacc, protoc-gen-go, linters, etc. ..."
# Set the env system-wide for later usage, e.g. build source
go env -w GO111MODULE=on
if [ "$FIND_FASTED_MIRROR" == "true" ]; then
    go env -w GOPROXY="$(find_fastest_go_proxy)"
else
    go env -w GOPROXY="https://goproxy.io"
fi

go get \
   github.com/golang/protobuf/protoc-gen-go@v1.3.3 \
   golang.org/x/lint/golint \
   golang.org/x/tools/cmd/goyacc \
   golang.org/x/tools/cmd/cover \
   github.com/mattn/goveralls \
   github.com/rakyll/gotest \
   github.com/wangkuiyi/goyaccfmt \
   github.com/wangkuiyi/ipynb/markdown-to-ipynb
cp "$GOPATH"/bin/* /usr/local/bin/


echo "Install protoc ..."
PROTOC_SITE="https://github.com/protocolbuffers/protobuf/releases"
axel --quiet --output "p.zip" \
     $PROTOC_SITE"/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip"
unzip -qq p.zip -d /usr/local
rm p.zip


# We have mirrored some software on QiNiu cloud
# which is used to speed up the build process.
QINIU_BUCKET=http://cdn.sqlflow.tech

echo "Install gRPC for Java as a protobuf-compiler ..."
# c.f. https://stackoverflow.com/a/53982507/724872.
PROTOC_JAVA_SITE_1="$QINIU_BUCKET/protoc/protoc-gen-grpc-java-1.21.0-linux-x86_64"
PROTOC_JAVA_SITE_2="https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe"
axel --quiet --output /usr/local/bin/protoc-gen-grpc-java \
     $PROTOC_JAVA_SITE_1 $PROTOC_JAVA_SITE_2
chmod +x /usr/local/bin/protoc-gen-grpc-java

if [ "$FIND_FASTED_MIRROR" == "true" ]; then
    echo "Choose fastest Maven mirror ..."
    # Travis CI occasionally fails on the default Maven central repo.
    # Ref: https://github.com/sql-machine-learning/sqlflow/issues/1654
    mkdir -p "$HOME/.m2"
    find_fastest_maven_repo >"$HOME/.m2/settings.xml"
fi


echo "Install Java linter ..."
axel --quiet --output /usr/local/bin/google-java-format-1.6-all-deps.jar \
    "$QINIU_BUCKET/checkstyle/google-java-format-1.6-all-deps.jar" \
    "https://github.com/google/google-java-format/releases/download/google-java-format-1.6/google-java-format-1.6-all-deps.jar"
axel --quiet --output /usr/local/bin/google_checks.xml \
    "$QINIU_BUCKET/checkstyle/google_checks.xml" \
    "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/google_checks.xml"
axel --quiet --output /usr/local/bin/checkstyle-8.29-all.jar \
    "$QINIU_BUCKET/checkstyle/checkstyle-8.29-all.jar" \
    "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-8.29/checkstyle-8.29-all.jar"

echo "Install BARON mathematical programming solver ..."
axel --quiet --output /tmp/baron-lin64.zip \
    "http://cdn.sqlflow.tech/ci/baron-lin64.zip" \
    "https://minlp.com/downloads/xecs/baron/current/baron-lin64.zip"
unzip -qq /tmp/baron-lin64.zip -d /tmp/
mv /tmp/baron-lin64/baron /usr/bin
rm -rf /tmp/baron-lin64
rm -rf /tmp/baron-lin64.zip
