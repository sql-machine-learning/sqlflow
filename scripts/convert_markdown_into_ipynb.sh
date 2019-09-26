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

command -v go >/dev/null 2>&1
if [ $? -ne 0 ]; then
    echo >&2 "Please install go https://golang.org/doc/install#install"
    exit 1
fi

if [[ $GOPATH == "" ]]; then
    echo "Set GOPATH to ~/go"
    export GOPATH=~/go
fi

SRC_FOLDER=${SRC_FOLDER:-doc/tutorial}
DEST_FOLDER=${DEST_FOLDER-/workspace}

go get -u github.com/wangkuiyi/ipynb/markdown-to-ipynb

cur_path="$(cd "$(dirname "$0")" && pwd -P)"
cd $cur_path/../

# convert markdown to ipynb
for file in ${SRC_FOLDER}/*.md; do
    filename=$(basename -- "$file")
    $GOPATH/bin/markdown-to-ipynb --code-block-type=sql < $file > $DEST_FOLDER/${filename%.*}."ipynb"
    if [ $? -ne 0 ]; then
        echo >&2 "markdown-to-ipynb $file error"
        exit 1
    fi
done
