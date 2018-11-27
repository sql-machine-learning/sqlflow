#!/bin/bash

cd $GOPATH/src/work

go run server.go &
go run client.go
