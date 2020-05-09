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

echo "try to pull sqlflow/sqlflow:ci"
docker pull -q sqlflow/sqlflow:ci

# Exit for any error.
set -e

echo "build the devbox image sqlflow:dev"
(cd $TRAVIS_BUILD_DIR/docker/dev
 docker build --cache-from sqlflow/sqlflow:ci -t sqlflow:dev .)

echo "build SQLFlow from source into $TRAVIS_BUILD_DIR/build using sqlflow:dev"
(cd $TRAVIS_BUILD_DIR
 docker run --rm -it -v $TRAVIS_BUILD_DIR:/work -w /work sqlflow:dev)

echo "build sqlflow:ci byloading $TRAVIS_BUILD_DIR/build"
(cd $TRAVIS_BUILD_DIR
 docker build -t sqlflow:ci .)
