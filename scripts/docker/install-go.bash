#!/bin/bash

# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

curl --silent https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -

export GO111MODULE=on

go get github.com/golang/protobuf/protoc-gen-go@v1.3.3
go get golang.org/x/lint/golint
go get golang.org/x/tools/cmd/goyacc
go get golang.org/x/tools/cmd/cover
go get github.com/mattn/goveralls
go get github.com/rakyll/gotest
go get github.com/wangkuiyi/goyaccfmt
go get github.com/wangkuiyi/yamlfmt

cp "$GOPATH"/bin/* /usr/local/bin/
