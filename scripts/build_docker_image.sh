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
apt-get update && apt-get install -y mysql-server
mkdir -p /var/run/mysqld
mkdir -p /var/lib/mysql
chown mysql:mysql /var/run/mysqld
chown mysql:mysql /var/lib/mysql
mkdir -p /docker-entrypoint-initdb.d

# 5. Install odpscmd for submitting alps predict job with odps udf script
# TODO(Yancey1989): using gomaxcompute instead of the odpscmd command-line tool.
wget -q http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh/1557995455961/odpscmd_public.zip
unzip -qq odpscmd_public.zip -d /usr/local/odpscmd
ln -s /usr/local/odpscmd/bin/odpscmd /usr/local/bin/odpscmd
rm -rf odpscmd_public.zip

# 6. Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
mkdir -p $IPYTHON_STARTUP
mkdir -p /workspace
echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
echo 'get_ipython().magic(u"%reload_ext autoreload")' >> $IPYTHON_STARTUP/00-first.py
echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

# 7. install xgboost
pip install xgboost==0.90
# Re-enable this after Ant-XGBoost is ready.
# pip install xgboost-launcher==0.0.4



# 9. Install additional dependencies for ElasticDL, ElasticDL CLI, and build testing images
apt-get update && apt-get install -y docker.io sudo
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
git clone https://github.com/sql-machine-learning/elasticdl.git
cd elasticdl
git checkout eb93e2a48e6fe8f077c4937d8c0c5987faa9cf56
pip install -r elasticdl/requirements.txt
python setup.py install
cd ..
