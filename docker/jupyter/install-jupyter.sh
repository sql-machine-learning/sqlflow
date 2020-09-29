#!/bin/sh

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

set -ex

# install jupyterhub Python package so that this image can be used as jupyterhub
# singleuser notebook server, ref: https://github.com/jupyterhub/jupyterhub/tree/master/singleuser
# Install pandas pre-compiled apk, we do not want to build
# this python package locally because it relies on gcc and
# other build tools, which make the image very large
wget -q http://cdn.sqlflow.tech/alpine/py3-pandas-1.0.3-r0.apk
wget -q -P /etc/apk/keys/ http://cdn.sqlflow.tech/alpine/sqlflow-5ef80180.rsa.pub
apk add py3-pandas-1.0.3-r0.apk && rm py3-pandas-1.0.3-r0.apk
# Dependencies for jupyterhub
apk add py3-cryptography py3-ruamel.yaml.clib py3-requests

pip -q install \
    jupyterhub==1.1.0 \
    notebook==6.0.3 \
    sqlflow==0.14.0

# Load SQLFlow's Jupyter magic command
# automatically. c.f. https://stackoverflow.com/a/32683001.
IPYTHON_STARTUP="/root/.ipython/profile_default/startup/"
mkdir -p "$IPYTHON_STARTUP"
mkdir -p /workspace/jupyter
{ echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")';
  echo 'get_ipython().magic(u"%reload_ext autoreload")';
  echo 'get_ipython().magic(u"%autoreload 2")'; } \
    >> "$IPYTHON_STARTUP"/00-first.py

# Enable highlighting, see https://stackoverflow.com/questions/43641362
NOTEBOOK_DIR=$(python3 -c "print(__import__('notebook').__path__[0])")
CODE_MIRROR_MODE_PATH=$NOTEBOOK_DIR/static/components/codemirror/mode
mkdir -p "$HOME"/.jupyter/custom/
mkdir -p "$CODE_MIRROR_MODE_PATH"/sqlflow
# Depends on Docekrfile to COPY *.js to /js.
cp /jupyter/js/custom.js "$HOME"/.jupyter/custom/
cp /jupyter/js/sqlflow.js "$CODE_MIRROR_MODE_PATH"/sqlflow/