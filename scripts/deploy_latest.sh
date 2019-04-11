#!/bin/bash
set -e

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin
docker build -t sqlflow/sqlflow:latest -f ./Dockerfile $GOPATH/bin
docker push sqlflow/sqlflow:latest
