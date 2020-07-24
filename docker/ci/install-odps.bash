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

# Install odpscmd for submitting Alps predict job with ODPS UDF script.
# TODO(Yancey1989): using gomaxcompute instead of the odpscmd command-line tool.
#
# Travis CI often breaks due to the unstable official download link on
# Aliyun.  So, we manually mirrored the package on AWS and Qiniu and
# download from all these mirrors simultaneously using axel.
M1=http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh
M2="http://cdn.sqlflow.tech/aliyun"
M3="https://sqlflow-release.s3.ap-east-1.amazonaws.com/aliyun"
axel --quiet \
     $M1/1557995455961/odpscmd_public.zip \
     $M2/1557995455961/odpscmd_public.zip \
     $M3/1557995455961/odpscmd_public.zip
unzip -qq odpscmd_public.zip -d /usr/local/odpscmd
ln -s /usr/local/odpscmd/bin/odpscmd /usr/local/bin/odpscmd
rm -rf odpscmd_public.zip
