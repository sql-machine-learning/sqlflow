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

# Wait for the creation of file /work/mysql-inited.  The entrypoint
# of sqlflow:mysql should create this file on a bind mount of the host
# filesystem.  So, the container running this script should also bind
# mount the same host directory to /work.
# shellcheck disable=SC2162
while read i; do if [ "$i" = "mysql-inited" ]; then break; fi; done \
    < <(inotifywait  -e create,open --format '%f' --quiet /work --monitor)

echo "Start SQLFlow server ..."
sqlflowserver

