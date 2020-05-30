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

# TRAVIS_PULL_REQUEST is set to the pull request number if the current
# job is a pull request build, or false if it’s not.
echo "TRAVIS_PULL_REQUEST $TRAVIS_PULL_REQUEST"

# TRAVIS_BRANCH:
# - for push builds, or builds not triggered by a pull request, this
#   is the name of the branch.
# - for builds triggered by a pull request this is the name of the
#   branch targeted by the pull request.
# - for builds triggered by a tag, this is the same as the name of the
#   tag (TRAVIS_TAG).
echo "TRAVIS_BRANCH $TRAVIS_BRANCH"

# TRAVIS_TAG: If the current build is for a git tag, this variable is
# set to the tag’s name.
echo "TRAVIS_TAG $TRAVIS_TAG"

if [[ "$TRAVIS_PULL_REQUEST" != "false" ]]; then
    echo "Skip deployment on pull request"
    exit 0
fi

# Figure out the tag to push sqlflow:ci.
if [[ "$TRAVIS_BRANCH" == "develop" ]]; then
    if [[ "$TRAVIS_EVENT_TYPE" == "cron" ]]; then
        DOCKER_TAG="nightly"
    else
        DOCKER_TAG="latest"
    fi
elif [[ "$TRAVIS_TAG" != "" ]]; then
    DOCKER_TAG="$TRAVIS_TAG"
else
    echo "Cannot figure out Docker image tag."
    exit 1
fi

# Build sqlflow:dev and sqlflow:ci.
"$(dirname "$0")"/build.sh

# Build sqlflow:mysql
(cd "$TRAVIS_BUILD_DIR" && \
    docker build -t sqlflow:mysql -f docker/mysql/Dockerfile .)

# Build sqlflow:notebook
(cd "$TRAVIS_BUILD_DIR" && \
    docker build -t sqlflow/sqlflow:notebook -f docker/notebook/Dockerfile .)

echo "$DOCKER_PASSWORD" |
    docker login --username "$DOCKER_USERNAME" --password-stdin

echo "docker push sqlflow:dev ..."
docker tag sqlflow:dev sqlflow/sqlflow:dev
docker push sqlflow/sqlflow:dev

echo "docker push sqlflow/sqlflow:$TRAVIS_TAG ..."
docker tag sqlflow:ci sqlflow/sqlflow:"$DOCKER_TAG"
docker push sqlflow/sqlflow:"$DOCKER_TAG"

echo "docker push sqlflow/sqlflow:mysql"
docker tag sqlflow:mysql sqlflow/sqlflow:mysql
docker push sqlflow/sqlflow:mysql

echo "docker push sqlflow/sqlflow:notebook"
docker push sqlflow/sqlflow:notebook