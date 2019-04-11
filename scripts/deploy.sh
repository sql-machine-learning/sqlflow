#!/bin/bash
set -e

echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
# docker build -t sqlflow:nightly -f ./Dockerfile $GOPATH/bin
# docker tag sqlflow:nightly sqlflow/sqlflow:nightly
# docker push sqlflow/sqlflow:nightly
