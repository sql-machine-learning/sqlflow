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

# c.f. https://kubernetes.io/docs/tasks/tools/install-minikube/
MINIKUBE_RELEASE_SITE="https://storage.googleapis.com/minikube/releases"
axel --quiet --output minikube \
     $MINIKUBE_RELEASE_SITE/v$MINIKUBE_VERSION/minikube-linux-amd64
chmod a+x minikube
sudo mv minikube /usr/local/bin/minikube

# Kubernetes 1.18.2 requires conntrack.
sudo apt-get -qq install -y conntrack socat
