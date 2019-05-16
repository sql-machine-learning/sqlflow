#!/bin/bash
set -e

service mysql start

cat example/datasets/popularize_churn.sql | mysql -uroot -proot
cat example/datasets/popularize_iris.sql | mysql -uroot -proot
cat example/datasets/create_model_db.sql | mysql -uroot -proot

export SQLFLOW_TEST_DB=mysql

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_log_level=debug go test -v ./...  -covermode=count -coverprofile=coverage.out
