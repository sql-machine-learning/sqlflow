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

# This file depends on install-python.bash.
pip install --quiet \
    jupyter==1.0.0 \
    notebook==6.0.0 \
    sqlflow==0.10.0 # sqlflow is the Python client of SQLFlow server.

# Load SQLFlow's Jupyter magic command
# automatically. c.f. https://stackoverflow.com/a/32683001.
IPYTHON_STARTUP="/root/.ipython/profile_default/startup/"
mkdir -p "$IPYTHON_STARTUP"
mkdir -p /workspace
{ echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")';
  echo 'get_ipython().magic(u"%reload_ext autoreload")';
  echo 'get_ipython().magic(u"%autoreload 2")'; } \
    >> "$IPYTHON_STARTUP"/00-first.py

# Enable highlighting, see https://stackoverflow.com/questions/43641362
NOTEBOOK_DIR=$(python -c "print(__import__('notebook').__path__[0])")
CODE_MIRROR_MODE_PATH=$NOTEBOOK_DIR/static/components/codemirror/mode
mkdir -p "$HOME"/.jupyter/custom/
mkdir -p "$CODE_MIRROR_MODE_PATH"/sqlflow
# Depends on Docekrfile to COPY *.js to /js.
cp /jupyter/js/custom.js "$HOME"/.jupyter/custom/
cp /jupyter/js/sqlflow.js "$CODE_MIRROR_MODE_PATH"/sqlflow/
