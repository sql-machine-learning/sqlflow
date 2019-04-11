#!/bin/bash
set -e

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin
docker build -t sqlflow/sqlflow:nightly -f ./Dockerfile $GOPATH/bin
docker push sqlflow/sqlflow:nightly
