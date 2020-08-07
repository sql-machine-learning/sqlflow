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
if [[ "$(docker images -q sqlflow:dev 2> /dev/null)" == "" ]]; then
    echo "  using sqlflow/sqlflow:dev as the cache image"
    docker pull sqlflow/sqlflow:dev
    docker build --cache-from sqlflow/sqlflow:dev -t sqlflow:dev \
           --build-arg FIND_FASTED_MIRROR="$FIND_FASTED_MIRROR" \
           -f docker/dev/Dockerfile "$TRAVIS_BUILD_DIR"
else
    docker build -t sqlflow:dev --build-arg FIND_FASTED_MIRROR="$FIND_FASTED_MIRROR" \
           -f docker/dev/Dockerfile "$TRAVIS_BUILD_DIR"
fi

echo "Build SQLFlow into $TRAVIS_BUILD_DIR/build using sqlflow:dev ..."
mkdir -p "$TRAVIS_BUILD_DIR"/build
docker run --rm \
       -v "$TRAVIS_BUILD_DIR":/work -w /work \
       -v "$GOPATH":/root/go \
       -v "$HOME"/.m2:/root/.m2 \
       -v "$HOME"/.cache:/root/.cache \
       sqlflow:dev

echo "Build sqlflow:ci byloading $TRAVIS_BUILD_DIR/build ..."
docker build -t sqlflow:ci \
       -f docker/ci/Dockerfile "$TRAVIS_BUILD_DIR"


function build_sqlflow_image() {
    echo "Build sqlflow:${1} by loading $TRAVIS_BUILD_DIR/build ..."
    if docker pull sqlflow/sqlflow:"${1}" 2> /dev/null; then
       echo " using sqlflow/sqlflow:${1} as the cache image"
       docker build --cache-from sqlflow/sqlflow:"${1}" -t sqlflow:"${1}" \
              --build-arg FIND_FASTED_MIRROR="$FIND_FASTED_MIRROR" \
              -f docker/"${1}"/Dockerfile "${TRAVIS_BUILD_DIR}"
    else
       docker build -t sqlflow:"${1}" \
           --build-arg FIND_FASTED_MIRROR="$FIND_FASTED_MIRROR" \
           -f docker/"${1}"/Dockerfile "$TRAVIS_BUILD_DIR"
    fi
}

if [[ "$TRAVIS_BUILD_STAGE_NAME" != "Deploy" ]]; then
    echo "Skip build SQLFlow deployment Docker images"
    exit 0
fi

# Build SQLFlow componenets Docker images
build_sqlflow_image server
build_sqlflow_image mysql
build_sqlflow_image jupyter
build_sqlflow_image step
build_sqlflow_image modelzooserver

echo "Clean up root permission $TRAVIS_BUILD_DIR/build ..."
docker run --rm  -v "$TRAVIS_BUILD_DIR":/work sqlflow:ci rm -rf /work/build /work/java /work/python
