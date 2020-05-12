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

echo "Use Ubuntu apt-get source $APT_MIRROR ..."
cat > /etc/apt/sources.list <<EOF
deb $APT_MIRROR bionic main restricted universe multiverse
deb $APT_MIRROR bionic-security main restricted universe multiverse
deb $APT_MIRROR bionic-updates main restricted universe multiverse
deb $APT_MIRROR bionic-proposed main restricted universe multiverse
deb $APT_MIRROR bionic-backports main restricted universe multiverse
EOF
apt-get -qq update


echo "Install apt packages ..."
DOWNLOAD_TOOLS="curl unzip"
BUILD_ESSENTIAL="build-essential git"
PYTHON_DEV="python3-dev python3-pip" # Many pip packages require Python.h
JAVA_DEV="openjdk-8-jdk maven"
SHELL_LINTER="shellcheck"
apt-get -qq install -y --no-install-recommends \
        $DOWNLOAD_TOOLS \
        $BUILD_ESSENTIAL \
        $PYTHON_DEV \
        $JAVA_DEV \
        $SHELL_LINTER \
        > /dev/null
rm -rf /var/lib/apt/lists/*
apt-get -qq clean -y


echo "Make Python 3 the the default"
ln -s /usr/bin/python3 /usr/local/bin/python

echo "Upgrade pip and setuptools creates /usr/local/bin/pip ..."
# Update setuptools because
# https://github.com/red-hat-storage/ocs-ci/pull/971/files
pip3 install --quiet --upgrade pip setuptools six


echo "Install pip packages ..."
PRE_COMMIT="pre-commit==1.18.3"
PY_TEST="pytest==5.3.0"
JS_LINTER=jsbeautifier
PYTHON_LINTER="yapf isort pylint flake8"
WHEEL="wheel"
pip install --quiet \
    $WHEEL \
    $PRE_COMMIT \
    $PY_TEST \
    $JS_LINTER \
    $PYTHON_LINTER
rm -rf $HOME/.cache/pip/*


echo "Install Go compiler ..."
GO_DEV="https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz"
curl -sL $GO_DEV  | tar -C /usr/local -xzf -
export GOPATH="/root/go"
export PATH="/usr/local/go/bin:$GOPATH/bin:$PATH"


echo "Install goyacc, protoc-gen-go, linters, etc. ..."
export GO111MODULE=on
go get \
   github.com/golang/protobuf/protoc-gen-go@v1.3.3 \
   golang.org/x/lint/golint \
   golang.org/x/tools/cmd/goyacc \
   golang.org/x/tools/cmd/cover \
   github.com/mattn/goveralls \
   github.com/rakyll/gotest \
   github.com/wangkuiyi/goyaccfmt \
   github.com/wangkuiyi/yamlfmt \
   github.com/wangkuiyi/ipynb/markdown-to-ipynb
cp "$GOPATH"/bin/* /usr/local/bin/


echo "Install protoc ..."
curl -sL \
     "https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip" \
     -o p.zip
unzip -qq p.zip -d /usr/local
rm p.zip


echo "Install gRPC for Java as a protobuf-compiler ..."
# c.f. https://stackoverflow.com/a/53982507/724872.
curl -sL \
     "https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe" \
     -o /usr/local/bin/protoc-gen-grpc-java
chmod +x /usr/local/bin/protoc-gen-grpc-java


echo "Use GCS based Maven-central mirror ..."
# Travis CI occasionally fails on the default maven central repo.
# Ref: https://github.com/sql-machine-learning/sqlflow/issues/1654
mkdir -p $HOME/.m2/
echo '<settings>
  <mirrors>
    <mirror>
      <id>google-maven-central</id>
      <name>GCS Maven Central mirror</name>
      <url>https://maven-central.storage-download.googleapis.com/maven2/</url>
      <mirrorOf>central</mirrorOf>
    </mirror>
  </mirrors>
</settings>' > $HOME/.m2/settings.xml


echo "Install Java linter ..."
curl -sLJ \
     "https://github.com/google/google-java-format/releases/download/google-java-format-1.6/google-java-format-1.6-all-deps.jar" \
     -o /usr/local/bin/google-java-format-1.6-all-deps.jar
curl -sLJ \
     "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/google_checks.xml" \
     -o /usr/local/bin/google_checks.xml
curl -sLJ \
     "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-8.29/checkstyle-8.29-all.jar" \
     -o /usr/local/bin/checkstyle-8.29-all.jar
