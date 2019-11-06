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

# We use Hadoop client to write CSV files to Hive tables.
HADOOP_URL=https://archive.apache.org/dist/hadoop/common/hadoop-${HADOOP_VERSION}/hadoop-${HADOOP_VERSION}.tar.gz
curl -fsSL "$HADOOP_URL" -o /tmp/hadoop.tar.gz
tar -xzf /tmp/hadoop.tar.gz -C /opt/
rm -rf /tmp/hadoop.tar.gz
rm -rf /opt/hadoop-${HADOOP_VERSION}/share/doc

# Configure HDFS namenode at localhost:8020.
echo '<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="configuration.xsl"?>
<configuration>
<property><name>hadoop.proxyuser.hue.hosts</name><value>*</value></property>
<property><name>fs.defaultFS</name><value>hdfs://localhost:8020</value></property>
<property><name>hadoop.proxyuser.hue.groups</name><value>*</value></property>
<property><name>hadoop.proxyuser.root.groups</name><value></value></property>
<property><name>hadoop.proxyuser.root.hosts</name><value></value></property>
<property><name>hadoop.http.staticuser.user</name><value>root</value></property>
</configuration>
' > /opt/hadoop-${HADOOP_VERSION}/etc/hadoop/core-site.xml