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

export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false
export MINIKUBE_HOME=$HOME
export CHANGE_MINIKUBE_NONE_USER=true
export KUBECONFIG=$HOME/.kube/config
export K8S_VERSION=1.14.0
export MINIKUBE_VERSION=1.1.1

# Install kubectl and minikube (currently only used for ElasticDL integration tests with maxcompute)
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v$K8S_VERSION/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
curl -Lo minikube https://storage.googleapis.com/minikube/releases/v$MINIKUBE_VERSION/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/
mkdir -p $HOME/.kube $HOME/.minikube
touch $KUBECONFIG
sudo minikube start --vm-driver=none --kubernetes-version=v$K8S_VERSION --cpus 2 --memory 6144
sudo chown -R travis: $HOME/.minikube/
kubectl cluster-info

cd scripts/katib_yaml

# Install kustomize
curl -s https://api.github.com/repos/kubernetes-sigs/kustomize/releases |\
  grep browser_download |\
  grep linux |\
  cut -d '"' -f 4 |\
  grep /kustomize/v |\
  sort | tail -n 1 |\
  xargs curl -O -L
tar xzf ./kustomize_v*_linux_amd64.tar.gz

cp kustomize crds/
cp kustomize controller/

# Install katib
kubectl apply -f katib-namespace.yaml

cd crds/
./kustomize build . | kubectl apply -f -
cd ../controller/
./kustomize build . | kubectl apply -f -

cd ../../../


