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

if [[ "$TRAVIS_OS_NAME" != "linux" ]]; then
    echo "$0 can run on Linux host only"
    exit 1
fi

echo "Install axel on Travis CI VM ..."
$(dirname $0)/install_axel.sh

echo "Export Kubernetes environment variables ..."
# NOTE: According to https://stackoverflow.com/a/16619261/724872,
# source is very necessary here.
source $(dirname $0)/export_k8s_vars.sh

echo "Install kubectl ..."
$(dirname $0)/install_kubectl.sh

echo "Install minikube ..."
$(dirname $0)/install_minikube.sh

echo "Configure minikube ..."
mkdir -p $HOME/.kube $HOME/.minikube
touch $KUBECONFIG

$(dirname $0)/start_minikube.sh
sudo chown -R travis: $HOME/.minikube/

$(dirname $0)/start_argo.sh
