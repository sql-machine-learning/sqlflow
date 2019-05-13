#!/bin/bash
set -e

ls -lah
docker build -t sqlflow:dev -f Dockerfile.dev .
docker run --rm -v $GOPATH/src:/go/src -w /go/src/github.com/sql-machine-learning/sqlflow sqlflow:dev pre-commit run -a

# NOTE: mount all $GOPATH into container, then after build and test,
# $GOPATH/bin will have expected binaries.
docker run --rm -v $GOPATH:/go -w /go/src/github.com/sql-machine-learning/sqlflow sqlflow:dev bash scripts/test.sh
