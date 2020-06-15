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

# If argo already installed, skip
if kubectl get namespace argo>/dev/null 2>&1; then
	echo "Argo is already installed."
	exit 0
fi

echo "Install Argo on minikube cluster ..."
kubectl create namespace argo

# Get argo config from QiNiu or github
wget -q -O /tmp/argo-v2.7.7.yaml \
	http://cdn.sqlflow.tech/argo-v2.7.7.yaml
kubectl apply -n argo -f /tmp/argo-v2.7.7.yaml
kubectl create rolebinding default-admin \
	--clusterrole=admin \
	--serviceaccount=default:default

