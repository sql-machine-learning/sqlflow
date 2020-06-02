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

# NOTE: require external exported HADOOP_VERSION.

# NOTE: Hadoop provides an official website to choose a proper URL for fast
# downloading. The recommended URL is inside the website $HADOOP_DYN_SITE.
# Here we use a spider-like code to retrieve the recommended URL from the
# website $HADOOP_DYN_SITE. The URL is inside something like:
# <a href="https://mirror.bit.edu.cn/apache/hadoop/common/hadoop-3.2.1/
#  hadoop-3.2.1.tar.gz">...</a>

HADOOP_DYN_SITE="https://www.apache.org/dyn/closer.cgi/hadoop/common/hadoop-$HADOOP_VERSION/hadoop-$HADOOP_VERSION.tar.gz"

HADOOP_HREF_PATTERN="href\=\"https:\/\/.*\/hadoop-$HADOOP_VERSION.tar.gz\""

HADOOP_URL="$(curl -s "$HADOOP_DYN_SITE" | \
    grep -m 1 -o -E "$HADOOP_HREF_PATTERN" | \
    awk -F'"' '{print $2}')"

echo "Download Hadoop from $HADOOP_URL ..."

axel --quiet --output /tmp/hadoop.tar.gz "$HADOOP_URL"
tar -xzf /tmp/hadoop.tar.gz -C /opt/
rm -rf /tmp/hadoop.tar.gz
rm -rf /opt/hadoop-"$HADOOP_VERSION"/share/doc

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
' > /opt/hadoop-"$HADOOP_VERSION"/etc/hadoop/core-site.xml
