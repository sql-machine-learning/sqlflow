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

# Exit for any error.
set -e

# Try to pull from DockerHub.com
docker pull sqlflow/sqlflow:ci

# Build the devbox image while seeing if there are layers can be reused.
(cd $TRAVIS_BUILD_DIR/docker/dev
 docker build --cache-from sqlflow/sqlflow:ci -t sqlflow:ci .)

# Build SQLFlow into $TRAVIS_BUILD_DIR/build using the devbox image.
docker run --rm -v $TRAVIS_BUILD_DIR:/work -w /work sqlflow:dev

# Load $TRAVIS_BUILD_DIR/build into the CI docker image
(cd $TRAVIS_BUILD_DIR
 docker build -t sqlflow:ci .)
