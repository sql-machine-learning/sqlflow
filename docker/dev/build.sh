#!/bin/bash
# shellcheck disable=SC2086,SC2231,SC2002
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

# FIXME(weiguoz): bring the shellcheck back: SC2086,SC2231,SC2002

# Exit for any error.
set -e

# This script assumes the root of the SQLFlow source tree is
# $SQLFLOWPATH, or $PWD.
SQLFLOWPATH=${SQLFLOWPATH:=$PWD}
cd $SQLFLOWPATH
SQLFLOW_BIN=$SQLFLOWPATH/build
echo "Build $SQLFLOWPATH into $SQLFLOW_BIN ..."

echo "Build sqlflowserver, sqlflow, and step into $SQLFLOW_BIN ..."
go generate ./...
GOBIN=$SQLFLOW_BIN go install ./...

echo "Build $SQLFLOWPATH/python/couler into $SQLFLOW_BIN ..."
cd $SQLFLOWPATH/python/couler
python setup.py bdist_wheel -q --dist-dir $SQLFLOW_BIN > /dev/null

echo "Build Fluid ..."
cd $SQLFLOW_BIN
if [[ ! -d fluid ]]; then
    git clone https://github.com/sql-machine-learning/fluid.git
fi
cd fluid
git checkout ceda474
python setup.py bdist_wheel -q --dist-dir $SQLFLOW_BIN > /dev/null

echo "Build parser gRPC servers in Java ..."
# Make mvn compile quiet
export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn"

cd $SQLFLOWPATH/java/parse-interface
mvn -B -q clean install # Write to local Maven repository.

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

echo "Generate Python protobuf ..."
protoc --python_out=$SQLFLOWPATH/python/runtime/dbapi/table_writer \
        -I $SQLFLOWPATH/go/proto/ sqlflow.proto

python -m grpc_tools.protoc \
    --python_out=$SQLFLOWPATH/python/runtime/model/ \
    --grpc_python_out=$SQLFLOWPATH/python/runtime/model/ \
    -I $SQLFLOWPATH/go/proto modelzooserver.proto

# A workaround for the issue: https://github.com/protocolbuffers/protobuf/issues/1491
sed -i 's/import modelzooserver_pb2/from . import modelzooserver_pb2/g' \
    $SQLFLOWPATH/python/runtime/model/modelzooserver_pb2_grpc.py

echo "Build model zoo ..."
cd $SQLFLOW_BIN
if [[ ! -d models ]]; then
    git clone https://github.com/sql-machine-learning/models.git
fi
cd models
git fetch origin # The residual local repo might not be on a branch.
git checkout 90e3b8b0de91faef942315a7dca7f94d175cec97
python setup.py bdist_wheel -q --dist-dir $SQLFLOW_BIN > /dev/null

echo "Convert tutorials from Markdown to IPython notebooks ..."
mkdir -p $SQLFLOW_BIN/tutorial
for file in $SQLFLOWPATH/doc/tutorial/*.md; do
    base=$(basename -- "$file")
    output=$SQLFLOW_BIN/tutorial/${base%.*}."ipynb"
    echo "Generate $output ..."
    cat $file | markdown-to-ipynb --code-block-type=sql > $output
done

echo "Build Finished!"
