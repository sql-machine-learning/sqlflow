#!/bin/bash
# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

service mysql start

go generate ./...
go get -v -t ./...
go install ./...

DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
sqlflowserver --datasource=${DATASOURCE} &
SQLFLOW_SERVER=localhost:50051 ipython sql/python/test_magic.py
