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

echo "Install Argo on minikube cluster ..."
kubectl create namespace argo
kubectl apply -n argo -f \
  https://raw.githubusercontent.com/argoproj/argo/v2.7.7/manifests/install.yaml
kubectl create rolebinding default-admin \
  --clusterrole=admin \
  --serviceaccount=default:default
