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
echo "TRAVIS_EVENT_TYPE $TRAVIS_EVENT_TYPE"

# TRAVIS_TAG: If the current build is for a git tag, this variable is
# set to the tag’s name.
echo "TRAVIS_TAG $TRAVIS_TAG"

# For github actions build, TRAVIS_PULL_REQUEST is "" when it is not a
# pull request build, so set it to false when it's empty.
if [[ "$TRAVIS_PULL_REQUEST" == "" ]]; then
    TRAVIS_PULL_REQUEST="false"
fi

# Early stop the process if it is a PR build
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

# Build sqlflow:dev, sqlflow:ci, and sqlflow:release.
"$(dirname "$0")"/build.sh

function push_image() {
    LOCAL_TAG=$1
    REMOTE_TAG=$2
    
    # push SQLFlow image to official Docker Hub
    echo "docker push sqlflow/sqlflow:$REMOTE_TAG ..."
    docker tag sqlflow:"$LOCAL_TAG" sqlflow/sqlflow:"$REMOTE_TAG"
    docker push sqlflow/sqlflow:"$REMOTE_TAG"
    docker rmi sqlflow/sqlflow:"$REMOTE_TAG"

    # push SQLFlow image to Aliyun Docker Hub
    echo "docker push registry.cn-hangzhou.aliyuncs.com/sql-machine-learning/sqlflow:$REMOTE_TAG  ..."
    docker tag sqlflow:"$LOCAL_TAG" registry.cn-hangzhou.aliyuncs.com/sql-machine-learning/sqlflow:"$REMOTE_TAG"
    docker push registry.cn-hangzhou.aliyuncs.com/sql-machine-learning/sqlflow:"$REMOTE_TAG"
    docker rmi registry.cn-hangzhou.aliyuncs.com/sql-machine-learning/sqlflow:"$REMOTE_TAG"
}

echo "$DOCKER_PASSWORD" |
    docker login --username "$DOCKER_USERNAME" --password-stdin

echo "$ALIYUN_DOCKER_PASSWORD" |
    docker login --username "$ALIYUN_DOCKER_USERNAME" --password-stdin registry.cn-hangzhou.aliyuncs.com

push_image dev dev
push_image ci "$DOCKER_TAG"
push_image mysql mysql
push_image jupyter jupyter
push_image server server
push_image step step
push_image modelzooserver modelzooserver
