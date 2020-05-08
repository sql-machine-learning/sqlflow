#!/bin/bash

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
GOBIN=$SQLFLOW_BIN go install -v ./...

# Build Couler
cd $SQLFLOWPATH/python/couler
python setup.py bdist_wheel
mv dist/*whl $SQLFLOW_BIN

# Build Fluid
git clone https://github.com/sql-machine-learning/fluid.git
cd fluid
git checkout ceda474
python setup.py bdist_wheel
mv dist/*.whl $SQLFLOW_BIN

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

# Convert tutorial markdown to ipython notebook
bash $SQLFLOWPATH/scripts/convert_markdown_into_ipynb.sh
