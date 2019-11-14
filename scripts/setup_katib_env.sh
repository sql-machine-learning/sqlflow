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

cd scripts

#install kustomize
curl -s https://api.github.com/repos/kubernetes-sigs/kustomize/releases |\
  grep browser_download |\
  grep linux |\
  cut -d '"' -f 4 |\
  grep /kustomize/v |\
  sort | tail -n 1 |\
  xargs curl -O -L
tar xzf ./kustomize_v*_linux_amd64.tar.gz
./kustomize version

git clone https://github.com/kubeflow/manifests.git

cp kustomize manifests/katib/katib-crds/base/
cp kustomize manifests/katib/katib-controller/base/

#install katib
kubectl apply -f katib-namespace.yaml

cd manifests/katib/katib-crds/base
./kustomize build . | kubectl apply -f -
cd ../../katib-controller/base
./kustomize build . | kubectl apply -f -

cd ../../../../../


