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

# Install ElasticDL and kubectl.
apt-get update && apt-get install -y docker.io sudo
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
git clone https://github.com/sql-machine-learning/elasticdl.git
cd elasticdl
git checkout eb93e2a48e6fe8f077c4937d8c0c5987faa9cf56 # TODO(terry): update later.
pip install -r elasticdl/requirements.txt
python setup.py install
cd ..