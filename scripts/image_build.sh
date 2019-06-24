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


# 0. Install miniconda and python and python dependencies.
curl -sL https://repo.continuum.io/miniconda/Miniconda3-latest-Linux-x86_64.sh -o mconda-install.sh
bash -x mconda-install.sh -b -p miniconda
rm mconda-install.sh
/miniconda/bin/conda create -y -q -n sqlflow-dev python=3.6 ${CONDA_ADD_PACKAGES}
echo ". /miniconda/etc/profile.d/conda.sh" >> ~/.bashrc
echo "source activate sqlflow-dev" >> ~/.bashrc

# keras.datasets.imdb only works with numpy==1.16.1
# Fix jupyter server "connecting to kernel" problem by installing tornado==4.5.3
# https://github.com/jupyter/notebook/issues/2664#issuecomment-468954423
source /miniconda/bin/activate sqlflow-dev && python -m pip install \
numpy==1.16.1 \
tensorflow==2.0.0-alpha0 \
mysql-connector-python \
impyla \
pyodps \
jupyter \
sqlflow==0.1.0 \
pre-commit \
tornado==4.5.3 \
${PIP_ADD_PACKAGES}

# 1. Install Go 1.11.5
wget --quiet https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.11.5.linux-amd64.tar.gz
rm go1.11.5.linux-amd64.tar.gz
mkdir -p /go

# 2. Install Go compile tools
go get github.com/golang/protobuf/protoc-gen-go
mv $GOPATH/bin/protoc-gen-go /usr/local/bin/
go get golang.org/x/lint/golint
mv $GOPATH/bin/golint /usr/local/bin

# 3. Install protobuf compiler
wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
unzip -qq protoc-3.7.1-linux-x86_64.zip -d /usr/local
rm protoc-3.7.1-linux-x86_64.zip

# 3.1 Install gRPC for Java as a protobuf-compiler plugin. c.f. https://stackoverflow.com/a/53982507/724872.
wget -q http://central.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe
mv protoc-gen-grpc-java-1.21.0-linux-x86_64.exe /usr/local/bin/protoc-gen-grpc-java
chmod +x /usr/local/bin/protoc-gen-grpc-java

# 4. Install mysql without a password prompt
echo 'mysql-server mysql-server/root_password password root' | debconf-set-selections
echo 'mysql-server mysql-server/root_password_again password root' | debconf-set-selections
apt-get install -y mysql-server
mkdir -p /var/run/mysqld
mkdir -p /var/lib/mysql
chown mysql:mysql /var/run/mysqld
chown mysql:mysql /var/lib/mysql
mkdir -p /docker-entrypoint-initdb.d

# 5. Build SQLFlow binaries by git clone the latest develop branch.
#    Then move binary file: "sqlflowserver" and "demo" to /usr/local/bin
#    Then delete contents under $GOPATH to reduce the image size.
# NOTE: During development and testing, /go will be overridden by -v.
mkdir -p /go/src/github.com/sql-machine-learning
cd /go/src/github.com/sql-machine-learning
git clone -q https://github.com/sql-machine-learning/sqlflow.git
cd sqlflow
go generate ./...
go get -t ./...
go install -v ./...
mv $GOPATH/bin/sqlflowserver /usr/local/bin
mv $GOPATH/bin/demo /usr/local/bin
cp -r $GOPATH/src/github.com/sql-machine-learning/sqlflow/sql/python/sqlflow_submitter /miniconda/envs/sqlflow-dev/lib/python3.6/site-packages/
rm -rf /go/src/*
rm -rf /go/bin/*
cd /

# 6. Install latest sqlflow_models for testing custom models, see main_test.go:CaseTrainCustomModel
git clone https://github.com/sql-machine-learning/models.git
cd models
bash -c "source activate sqlflow-dev && python setup.py install"
cd ..
rm -rf models

# 7. Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
mkdir -p $IPYTHON_STARTUP
mkdir -p /workspace
echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py
curl https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/example/jupyter/example.ipynb --output /workspace/example.ipynb
