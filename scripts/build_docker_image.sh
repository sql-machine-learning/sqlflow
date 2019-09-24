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

# keras.datasets.imdb only works with numpy==1.16.1
# NOTE: shap == 0.30.1 depends on dill but not include dill as it's dependency, need to install manually
# source /miniconda/bin/activate sqlflow-dev && python -m pip install \
install_python_deps() {
  python -m pip install \
  numpy==1.16.1 \
  tensorflow==${TENSORFLOW_VERSION} \
  mysqlclient \
  impyla \
  pyodps \
  jupyter \
  notebook==6.0.0 \
  sqlflow==0.5.0 \
  pre-commit \
  odps \
  dill \
  shap \
  xgboost==0.90 \
  ${PIP_ADD_PACKAGES}
}

# 1. Install Go 1.11.5
install_golang() {
  wget --quiet https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
  tar -C /usr/local -xzf go1.11.5.linux-amd64.tar.gz
  rm go1.11.5.linux-amd64.tar.gz
  mkdir -p /go

  # Install Go compile tools
  go get github.com/golang/protobuf/protoc-gen-go
  mv $GOPATH/bin/protoc-gen-go /usr/local/bin/
  go get golang.org/x/lint/golint
  mv $GOPATH/bin/golint /usr/local/bin

  # Install protobuf compiler
  wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
  unzip -qq protoc-3.7.1-linux-x86_64.zip -d /usr/local
  rm protoc-3.7.1-linux-x86_64.zip
  # Install gRPC for Java as a protobuf-compiler plugin. c.f. https://stackoverflow.com/a/53982507/724872.
  wget -q http://central.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe
  mv protoc-gen-grpc-java-1.21.0-linux-x86_64.exe /usr/local/bin/protoc-gen-grpc-java
  chmod +x /usr/local/bin/protoc-gen-grpc-java
}

# 2. Install mysql without a password prompt
install_mysql() {
  apt-get install -y default-mysql-server
  mkdir -p /var/run/mysqld
  mkdir -p /var/lib/mysql
  chown mysql:mysql /var/run/mysqld
  chown mysql:mysql /var/lib/mysql
  # debian's default-mysql-server install a mariadb and to set root password
  # we need to use mysqld_safe to start the server and set the root password
  mysqld_safe --skip-networking &
  sleep 5
  mysql -uroot -proot -e "ALTER USER 'root'@'localhost' IDENTIFIED BY 'root';"
  kill `cat /var/run/mysqld/mysqld.pid`
  mkdir -p /docker-entrypoint-initdb.d
}

# 3. Install latest sqlflow_models for testing custom models, see main_test.go:CaseTrainCustomModel
# NOTE: The sqlflow_models works well on the specific Tensorflow version,
#       we can skip installing sqlflow_models if using the older Tensorflow.
install_sqlflow_models() {
  if [ "${WITH_SQLFLOW_MODELS:-ON}" = "ON" ]; then
    git clone https://github.com/sql-machine-learning/models.git
    cd models
    bash -c "python setup.py install"
    cd ..
    rm -rf models
  fi
}

# 4. Install odpscmd for submitting alps predict job with odps udf script
# TODO(Yancey1989): using gomaxcompute instead of the odpscmd command-line tool.
install_odpscmd() {
  wget -q http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh/1557995455961/odpscmd_public.zip
  unzip -qq odpscmd_public.zip -d /usr/local/odpscmd
  ln -s /usr/local/odpscmd/bin/odpscmd /usr/local/bin/odpscmd
  rm -rf odpscmd_public.zip
}

# 5. Load sqlflow Jupyter magic command automatically under /workspace. 
#    c.f. https://stackoverflow.com/a/32683001.
install_magic_command() {
  mkdir -p $IPYTHON_STARTUP
  mkdir -p /workspace
  echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
  echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py
}


# 6. install Hadoop to use as the client when writing CSV to hive tables
install_hadoop() {
  HADOOP_URL=https://archive.apache.org/dist/hadoop/common/hadoop-${HADOOP_VERSION}/hadoop-${HADOOP_VERSION}.tar.gz
  curl -fsSL "$HADOOP_URL" -o /tmp/hadoop.tar.gz
  tar -xzf /tmp/hadoop.tar.gz -C /opt/
  rm -rf /tmp/hadoop.tar.gz
  rm -rf /opt/hadoop-${HADOOP_VERSION}/share/doc
}

# 7. Install additional dependencies for ElasticDL, ElasticDL CLI, and build testing images
install_elasticdl_deps() {
  apt-get install -y docker.io sudo
  curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
  # TODO(terrytangyuan): Uncomment once ElasticDL is open sourced
  # git clone https://github.com/wangkuiyi/elasticdl.git
  # cd elasticdl
  # pip install -r elasticdl/requirements.txt
  # python setup.py install
  # docker build -t elasticdl:dev -f elasticdl/docker/Dockerfile.dev .
  # docker build -t elasticdl:ci -f elasticdl/docker/Dockerfile.ci .
  # cd ..
}

install_python_deps
install_golang
install_mysql
install_sqlflow_models
install_odpscmd
install_magic_command
install_hadoop
install_elasticdl_deps
