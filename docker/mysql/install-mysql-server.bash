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

source find_fastest_resources.sh
choose_fastest_apt_source

echo "Install MySQL server without a password prompt ..."
echo 'mysql-server mysql-server/root_password password root' | \
    debconf-set-selections
echo 'mysql-server mysql-server/root_password_again password root' | \
    debconf-set-selections
apt-get -qq update > /dev/null
apt-get -qq install -y mysql-server > /dev/null
mkdir -p /var/run/mysqld
mkdir -p /var/lib/mysql
chown mysql:mysql /var/run/mysqld
chown mysql:mysql /var/lib/mysql
mkdir -p /docker-entrypoint-initdb.d
