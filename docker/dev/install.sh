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

DOWNLOAD_TOOLS="curl unzip"
BUILD_ESSENTIAL="build-essential git"
MYSQL_CLIENT="libmysqlclient-dev"
apt-get -qq install -y \
        $DOWNLOAD_TOOLS \
        $BUILD_ESSENTIAL \
        shellcheck \
	$MYSQL_CLIENT


# Install protoc
curl -sL \
     https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip \
     > p.zip
unzip -qq p.zip -d /usr/local
rm p.zip
