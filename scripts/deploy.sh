#!/bin/bash
# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

echo "TRAVIS_PULL_REQUEST $TRAVIS_PULL_REQUEST"
if [[ "$TRAVIS_PULL_REQUEST" != "false" ]]; then
    echo "skip deployment on pull request"
    exit 0
fi

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin
#docker build -t sqlflow/sqlflow:deploy_build -f ./Dockerfile .

echo "TRAVIS_BRANCH $TRAVIS_BRANCH"
if [[ "TRAVIS_BRANCH" == "develop" ]]; then
    if [[ $TRAVIS_EVENT_TYPE == "cron" ]]; then
        DOCKER_TAG="nightly"
    else
        DOCKER_TAG="latest"
    fi

    echo "docker push sqlflow/sqlflow:$DOCKER_TAG"
#    docker tag sqlflow/sqlflow:deploy_build sqlflow/sqlflow:$DOCKER_TAG
#    docker push sqlflow/sqlflow:$DOCKER_TAG
else
    echo "tag $TRAVIS_TAG"
    if [[ "$TRAVIS_TAG" != "" ]]; then
        echo "docker push sqlflow/sqlflow:$TRAVIS_TAG"
#        docker tag sqlflow/sqlflow:deploy_build sqlflow/sqlflow:$TRAVIS_TAG
#        docker push sqlflow/sqlflow:$TRAVIS_TAG
    fi
fi

