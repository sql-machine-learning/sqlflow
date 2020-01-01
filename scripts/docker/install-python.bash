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

apt-get install -y python3 python3-pip # Install /usr/bin/python3 and /usr/bin/pip3
ln -s /usr/bin/python3 /usr/local/bin/python
pip3 install --upgrade pip # Upgrade and creates /usr/local/bin/pip

# pip install mysqlclient needs GCC.
apt-get install -y build-essential

# keras.datasets.imdb only works with numpy==1.16.1
# NOTE: shap == 0.30.1 depends on dill but not include dill as it's dependency, need to install manually.
# NOTE: mysqlclient depends on apt-get install mysqlclient in install-mysql.bash.
pip install \
numpy==1.16.1 \
tensorflow==2.0.0b1 \
mysqlclient==1.4.4 \
impyla==0.16.0 \
pyodps==0.8.3 \
jupyter==1.0.0 \
notebook==6.0.0 \
sqlflow==0.9.0 \
pre-commit==1.18.3 \
dill==0.3.0 \
shap==0.30.1 \
xgboost==0.90 \
pytest==5.3.0 \
oss2==2.9.0 \
plotille==3.7 \
seaborn==0.9.0
