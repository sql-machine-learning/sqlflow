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

# When we do development locally, we might have already built
# sqflow:dev.  In this case, let us try to reuse it.  At CI time, we
# use sqlflow/sqlflow:dev as the cache when building sqlflow:dev on
# the newly started VM.
echo "Build the devbox image sqlflow:dev ..."
cd $TRAVIS_BUILD_DIR/docker/dev
if [[ "$(docker images -q sqlflow:dev 2> /dev/null)" == "" ]]; then
    echo "  using sqlflow/sqlflow:dev as the cache image"
    docker pull sqlflow/sqlflow:dev
    docker build --cache-from sqlflow/sqlflow:dev -t sqlflow:dev .
else
    docker build -t sqlflow:dev .
fi

echo "Build SQLFlow into $TRAVIS_BUILD_DIR/build using sqlflow:dev ..."
mkdir -p $TRAVIS_BUILD_DIR/build
docker run --rm -it \
       -v $TRAVIS_BUILD_DIR:/work -w /work \
       -v $GOPATH:/root/go \
       -v $HOME/.m2:/root/.m2 \
       -v $HOME/.cache:/root/.cache \
       sqlflow:dev

echo "Build sqlflow:ci byloading $TRAVIS_BUILD_DIR/build ..."
cd $TRAVIS_BUILD_DIR
docker build -t sqlflow:ci .
