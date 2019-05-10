#!/bin/bash
set -e

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin

if [[ $TRAVIS_EVENT_TYPE == "cron" ]]; then
    DOCKER_TAG="nightly"
else
    DOCKER_TAG="latest"
fi

docker build -t sqlflow/sqlflow:$DOCKER_TAG -f ./Dockerfile $GOPATH/bin
docker push sqlflow/sqlflow:$DOCKER_TAG

docker tag sqlflow:dev sqlflow/sqlflow:dev
docker push sqlflow/sqlflow:dev

docker build -t sqlflow/sqlflow:quickstart -f ./Dockerfile.quickstart .
docker push sqlflow/sqlflow:quickstart
