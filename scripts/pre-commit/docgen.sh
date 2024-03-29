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

# Don't use go run; instead, use go install and run the built
# executable file.  Otherwise, we'd get the following error as
# described in https://github.com/golang/go/issues/27215
#
# build sqlflow.org/sqlflow/go/cmd/docgen: cannot load
# sqlflow.org/sqlflow/pkg/sql/ir: no matching versions for query
# "latest"
#
go install ./go/cmd/docgen
docgen > doc/model_parameter.md
