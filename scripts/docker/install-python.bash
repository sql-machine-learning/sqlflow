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

# pip install mysqlclient needs GCC.
apt-get -qq update && apt-get -qq install -y build-essential

# We use miniconda to maintain the Python environment so we can install SQLFlow's submitter 
# template Python files to the canonical path /miniconda/envs/sqlflow-dev/lib/python3.6/site-packages/.
curl -sL https://repo.continuum.io/miniconda/Miniconda3-latest-Linux-x86_64.sh -o mconda-install.sh
bash -x mconda-install.sh -b -p miniconda
rm mconda-install.sh

/miniconda/bin/conda create -y -q -n sqlflow-dev python=3.6
echo ". /miniconda/etc/profile.d/conda.sh" >> ~/.bashrc
echo "source activate sqlflow-dev" >> ~/.bashrc

# keras.datasets.imdb only works with numpy==1.16.1
# NOTE: shap == 0.30.1 depends on dill but not include dill as it's dependency, need to install manually.
# NOTE: mysqlclient depends on apt-get install mysqlclient in install-mysql.bash.
source /miniconda/bin/activate sqlflow-dev && python -m pip -q install \
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
