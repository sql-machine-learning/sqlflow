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

apt-get -qq update && apt-get install -y openjdk-8-jdk maven > /dev/null

# Install gRPC for Java as a protobuf-compiler plugin. c.f. https://stackoverflow# .com/a/53982507/724872.
wget -q http://central.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe
mv protoc-gen-grpc-java-1.21.0-linux-x86_64.exe /usr/local/bin/protoc-gen-grpc-java
chmod +x /usr/local/bin/protoc-gen-grpc-java

# Use GCS based maven-central mirror.
# Travis CI occasionally fails on the default maven central repo.
# Ref: https://github.com/sql-machine-learning/sqlflow/issues/1654
mkdir -p /root/.m2/
echo '<settings>
  <mirrors>
    <mirror>
      <id>google-maven-central</id>
      <name>GCS Maven Central mirror</name>
      <url>https://maven-central.storage-download.googleapis.com/maven2/</url>
      <mirrorOf>central</mirrorOf>
    </mirror>
  </mirrors>
</settings>' > /root/.m2/settings.xml

curl -sLJ "https://github.com/google/google-java-format/releases/download/google-java-format-1.6/google-java-format-1.6-all-deps.jar" -o /usr/local/bin/google-java-format-1.6-all-deps.jar
