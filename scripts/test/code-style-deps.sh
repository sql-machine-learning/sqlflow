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

mkdir -p build

pip install pre-commit flake8 jsbeautifier
sudo apt-get update
sudo apt-get install -y shellcheck

# go linters
go install golang.org/x/lint/golint@latest
go install golang.org/x/tools/cmd/goyacc@latest
go install golang.org/x/tools/cmd/cover@latest
go install github.com/mattn/goveralls@latest
go install github.com/rakyll/gotest@latest
go install github.com/wangkuiyi/goyaccfmt@latest

# java linters
wget -q -O build/protoc-gen-grpc-java https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe
chmod +x build/protoc-gen-grpc-java
sudo mkdir -p /usr/local/bin
sudo mv build/protoc-gen-grpc-java /usr/local/bin/
wget -q -O build/google-java-format-1.6-all-deps.jar https://github.com/google/google-java-format/releases/download/google-java-format-1.6/google-java-format-1.6-all-deps.jar
sudo mv build/google-java-format-1.6-all-deps.jar /usr/local/bin
wget -q -O build/google_checks.xml https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/google_checks.xml
sudo mv build/google_checks.xml /usr/local/bin
wget -q -O build/checkstyle-8.29-all.jar https://github.com/checkstyle/checkstyle/releases/download/checkstyle-8.29/checkstyle-8.29-all.jar
sudo mv build/checkstyle-8.29-all.jar /usr/local/bin
