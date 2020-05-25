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

# Define a list of mirrors without using Bash arrays.
# c.f. https://stackoverflow.com/a/23930212/724872
read -r -d '' MAVEN_REPOS <<EOM
https://repo1.maven.org/maven2/
https://maven.aliyun.com/repository/central
EOM

# Find the fastest APT source using ping.
SPEED=99999.9
for i in $MAVEN_REPOS; do
    repo=`echo "$i" | cut -d '/' -f3`
    # c.f. https://stackoverflow.com/a/9634982/724872
    echo "Testig speed of $repo ..." >&2
    CUR_SPEED=$(ping -c 4 "$repo" | tail -1 | awk '{print $4}' | cut -d '/' -f 2)
    echo "$CUR_SPEED" >&2

    # c.f. https://stackoverflow.com/a/31087503/724872
    if (( $(echo "$CUR_SPEED < $SPEED" | bc -l) )); then
        BEST_REPO=$repo
        BEST_URL=$i
        SPEED="$CUR_SPEED"
    fi
done

# Write APT source lists.
cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd">
    <mirrors>
      <mirror>
          <id>${BEST_REPO}</id>
          <mirrorOf>central</mirrorOf>
          <name>${BEST_REPO}</name>
          <url>${BEST_URL}</url>
      </mirror>
    </mirrors>
</settings>
EOF

