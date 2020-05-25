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
DEFAULT_URL="https://pkg.go.dev/"
read -r -d '' GO_PROXIES <<EOM
${DEFAULT_URL}
https://goproxy.cn/,direct
EOM

# Find the fastest Go proxy using ping.
SPEED=99999.9
for i in $GO_PROXIES; do
    domain=`echo $i | cut -d '/' -f3`
    # c.f. https://stackoverflow.com/a/9634982/724872
    echo "Testig speed of $domain ..." >&2
    CUR_SPEED=$(ping -c 4 "$domain" | tail -1 | awk '{print $4}' | cut -d '/' -f 2)
    echo "$CUR_SPEED" >&2

    # c.f. https://stackoverflow.com/a/31087503/724872
    if (( $(echo "$CUR_SPEED < $SPEED" | bc -l) )); then
        BEST_URL=$i
        SPEED="$CUR_SPEED"
    fi
done

# If best is default site, return empty proxy, ohterwise
# return best proxy url
if [[ "${BEST_URL}" == "${DEFAULT_URL}" ]]; then
    echo ""
else
    echo "$BEST_URL"
fi

