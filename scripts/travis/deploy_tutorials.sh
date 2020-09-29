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

# This script push our tutorials to QiNiu Cloud, these tutorials
# can then be used by DSW (https://dsw-dev.data.aliyun.com/).

# For more informaiton about deployment with Travis CI, please refer
# to the file header of deploy_docker.sh

# For github actions build, TRAVIS_PULL_REQUEST is "" when it is not a
# pull request build, so set it to false when it's empty.
if [[ "$TRAVIS_PULL_REQUEST" == "" ]]; then
    TRAVIS_PULL_REQUEST="false"
fi

echo "TRAVIS_PULL_REQUEST $TRAVIS_PULL_REQUEST"
echo "TRAVIS_BRANCH $TRAVIS_BRANCH"

if [[ "$TRAVIS_PULL_REQUEST" != "false" ]]; then
    echo "Skip deployment on pull request"
    exit 0
fi

if [[ "$TRAVIS_BRANCH" != "develop" &&  "$TRAVIS_EVENT_TYPE" != "cron" ]]; then
    echo "Skip tutorial deployment on non-nightly build"
    exit 0
fi

echo "Install markdown-to-ipynb tool ..."
if [ "$GOPATH" == "" ]; then
    export GOPATH="/tmp/go"
fi
go get github.com/wangkuiyi/ipynb/markdown-to-ipynb > /dev/null
export PATH=$GOPATH/bin:$PATH

echo "Convert tutorials from Markdown to IPython notebooks ..."
cd "$TRAVIS_BUILD_DIR"
mkdir -p build/tutorial
for file in doc/tutorial/*.md; do
    base=$(basename -- "$file")
    output=build/tutorial/${base%.*}."ipynb"
    echo "Generate $output ..."
    markdown-to-ipynb --code-block-type=sql < "$file" > "$output"
done

echo "Publish /build/tutorial to Qiniu Object Storage ..."
F="qshell-linux-x64-v2.4.1"
if [[ ! -f "$F" ]]; then
    wget http://devtools.qiniu.com/$F.zip
    unzip $F.zip
fi
export PATH=$PWD:$PATH
$F account "$QINIU_AK" "$QINIU_SK" "wu"

for path in build/tutorial/*.ipynb; do
    retry=0
    file=$(basename "$path")
    while [[ $retry -lt 5 ]]; do
        if $F rput --overwrite \
                sqlflow-release-na \
                "sqlflow/tutorials/latest/$file" \
                "$path"; then
            break
        fi
    retry=$(( retry + 1 ))
    sleep 3
    done
done
