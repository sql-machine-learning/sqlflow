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

# Exit for any error.
set -e

# This script assumes the root of the SQLFlow source tree is
# $SQLFLOWPATH, or $PWD.
SQLFLOWPATH=${SQLFLOWPATH:=$PWD}
cd $SQLFLOWPATH

# The directory saving binaries
SQLFLOW_BIN=$SQLFLOWPATH/build

# Build sqlflowserver, sqlflow, and step into $SQLFLOW_BIN
go generate ./...
GOBIN=$SQLFLOW_BIN go install ./...

# Build Couler
cd $SQLFLOWPATH/python/couler
python setup.py bdist_wheel --dist-dir $SQLFLOW_BIN

# Build Fluid
cd $SQLFLOW_BIN
if [[ ! -d fluid ]]; then
    git clone https://github.com/sql-machine-learning/fluid.git
fi
cd fluid
git checkout ceda474
python setup.py bdist_wheel --dist-dir $SQLFLOW_BIN

# Build parser gRPC servers in Java.
# Make mvn compile quiet
export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn"

cd $SQLFLOWPATH/java/parse-interface
mvn clean install -B # Write to local Maven repository.

cd $SQLFLOWPATH/java/parser-hive
mvn -B -q clean compile assembly:single
mv target/*.jar $SQLFLOW_BIN

cd $SQLFLOWPATH/java/parser-calcite
mvn -B -q clean compile assembly:single
mv target/*.jar $SQLFLOW_BIN

cd $SQLFLOWPATH/java/parser
protoc --java_out=src/main/java \
       --grpc-java_out=src/main/java/ \
       --proto_path=src/main/proto/ \
       src/main/proto/parser.proto
mvn -B -q clean compile assembly:single
cp target/*.jar $SQLFLOW_BIN

# Build model zoo.
cd $SQLFLOW_BIN
if [[ ! -d models ]]; then
    git clone https://github.com/sql-machine-learning/models
fi
cd models
git checkout c897963f821d515651de79cb4ef1fbf6126ecaa5
python setup.py bdist_wheel --dist-dir $SQLFLOW_BIN

# Convert tutorials from Markdown to IPython notebooks.
mkdir -p $SQLFLOW_BIN/tutorial
for file in $SQLFLOWPATH/doc/tutorial/*.md; do
    base=$(basename -- "$file")
    output=$SQLFLOW_BIN/tutorial/${base%.*}."ipynb"
    cat $file | markdown-to-ipynb --code-block-type=sql > $output
done
