#!/bin/bash
service mysql start &

# TODO(tony): wait until mysql server is up

go test -v ./...