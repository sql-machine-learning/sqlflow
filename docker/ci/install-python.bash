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

apt-get -qq install -y python3 python3-pip > /dev/null
ln -s /usr/bin/python3 /usr/local/bin/python
ln -s /usr/bin/pip3 /usr/local/bin/pip

# Upgrade pip would creates /usr/local/bin/pip.  Update setuptools
# because https://github.com/red-hat-storage/ocs-ci/pull/971/files
pip3 install --quiet --upgrade pip setuptools six
rm -rf $HOME/.cache/pip/*
