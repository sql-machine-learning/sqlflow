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
read -r -d '' APT_SOURCES << EOM
mirrors.aliyun.com
mirrors.ustc.edu.cn
mirrors.163.com
archive.ubuntu.com
EOM

# Find the fastest APT source using ping.
SPEED=99999.9
for i in $APT_SOURCES; do
    # c.f. https://stackoverflow.com/a/9634982/724872
    echo "Testig speed of $i ..."
    CUR_SPEED=$(ping -c 4 "$i" | tail -1| awk '{print $4}' | cut -d '/' -f 2)
    echo "$CUR_SPEED"

    # c.f. https://stackoverflow.com/a/31087503/724872
    if (( $(echo "$CUR_SPEED < $SPEED" | bc -l) )); then
        BEST_APT_SOURCE="$i"
        SPEED="$CUR_SPEED"
    fi
done

# The default Ubuntu version is 18.04, code named bionic.
UBUNTU_CODENAME=${UBUNTU_CODENAME-"bionic"}

# Write APT source lists.
cat <<EOF
deb http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME main restricted universe multiverse
deb http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-security main restricted universe multiverse
deb http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-updates main restricted universe multiverse
deb http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-proposed main restricted universe multiverse
deb http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-backports main restricted universe multiverse
deb-src http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME main restricted universe multiverse
deb-src http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-security main restricted universe multiverse
deb-src http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-updates main restricted universe multiverse
deb-src http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-proposed main restricted universe multiverse
deb-src http://$BEST_APT_SOURCE/ubuntu/ $UBUNTU_CODENAME-backports main restricted universe multiverse
EOF
