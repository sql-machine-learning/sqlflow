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

cd java/parser

# Make downloading quiet.
# Downloading logs is about 6k lines, which makes viewing TravisCI log difficult
export MAVEN_OPTS=-Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn

# Generate GRPC & Protocol Buffer files
protoc --java_out=src/main/java --grpc-java_out=src/main/java/ --proto_path=src/main/proto/ src/main/proto/Parser.proto

# -B means batch mode, looks like batch mode is required to make downloading quiet
mvn test -B

