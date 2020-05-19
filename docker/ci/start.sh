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

SQLFLOW_NOTEBOOK_DIR=${SQLFLOW_NOTEBOOK_DIR:-/workspace}

function setup_sqlflow_notebook() {
    cd "$SQLFLOW_NOTEBOOK_DIR" ||
        ( echo "Cannot cd to $SQLFLOW_NOTEBOOK_DIR"; exit 1 )
  DS="mysql://root:root@tcp(${SQLFLOW_MYSQL_HOST}:${SQLFLOW_MYSQL_PORT})/?maxAllowedPacket=0"
  echo "Connect to the datasource $DS ..."
  SQLFLOW_DATASOURCE="$DS" SQLFLOW_SERVER="localhost:50051" \
                    jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root \
                    --NotebookApp.token=''
  cd ..
}

sqlflowserver &
setup_sqlflow_notebook
