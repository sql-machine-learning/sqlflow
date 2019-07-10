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

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin

if [[ $TRAVIS_EVENT_TYPE == "cron" ]]; then
    DOCKER_TAG="nightly"
    DOCKER_TAG_OLDER_TF="nightly-tf1.13.1"
else
    DOCKER_TAG="latest"
    DOCKER_TAG_OLDER_TF="latest-tf1.13.1"
fi

docker build -t sqlflow/sqlflow:$DOCKER_TAG -f ./Dockerfile .
docker push sqlflow/sqlflow:$DOCKER_TAG

docker build --build-arg TENSORFLOW_VERSION="1.13.1" -t sqlflow/sqlflow:$DOCKER_TAG_OLDER_TF -f ./Dockerfile .
docker push sqlflow/sqlflow:$DOCKER_TAG_OLDER_TF



