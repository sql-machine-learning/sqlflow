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

# NOTE: install-python.bash installs the Jupyter server.  Here we install only the magic command.
 
# Load SQLFlow's Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
mkdir -p $IPYTHON_STARTUP
mkdir -p /workspace
echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
echo 'get_ipython().magic(u"%reload_ext autoreload")' >> $IPYTHON_STARTUP/00-first.py
echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

# Enable highlighting, see https://stackoverflow.com/questions/43641362
mkdir -p $HOME/.jupyter/custom/
cp scripts/docker/js/custom.js $HOME/.jupyter/custom/
CODE_MIRROR_MODE_PATH=`python -c "print(__import__('notebook').__path__[0])"`/static/components/codemirror/mode
mkdir -p $CODE_MIRROR_MODE_PATH/sqlflow
cp scripts/docker/js/sqlflow.js $CODE_MIRROR_MODE_PATH/sqlflow/
