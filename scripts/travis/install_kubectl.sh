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

# Travis CI VMs and Vagrant provisioning allow sudo without password.
K8S_RELEASE_SITE="https://storage.googleapis.com/kubernetes-release/release"
axel --quiet --output kubectl \
     "$K8S_RELEASE_SITE"/v"$K8S_VERSION"/bin/linux/amd64/kubectl
chmod a+x kubectl
sudo mv kubectl /usr/local/bin/kubectl
