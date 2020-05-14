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

echo "Install Docker and sudo ..."
apt-get -qq install -y docker.io sudo > /dev/null

echo "Build and install ElasticDL ..."
git clone https://github.com/sql-machine-learning/elasticdl.git
cd elasticdl
git checkout eb93e2a48e6fe8f077c4937d8c0c5987faa9cf56 # TODO(terry): update later.
pip install --quiet -r elasticdl/requirements.txt
python setup.py -q install
cd ..
