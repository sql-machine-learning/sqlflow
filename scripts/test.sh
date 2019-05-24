#!/bin/bash
set -e

service mysql start

export SQLFLOW_TEST_DB=mysql

python -c "import sqlflow_models"

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_log_level=debug go test -v ./...  -covermode=count -coverprofile=coverage.out

python -m unittest discover -v sql/python "*_test.py"
