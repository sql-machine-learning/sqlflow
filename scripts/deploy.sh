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

# NOTE: this script should be called by .travis.yml
pushd ./example/
docker build -t sqlflow/quickstart:$DOCKER_TAG
docker push sqlflow/quickstart:$DOCKER_TAG
popd
