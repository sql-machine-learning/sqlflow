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

export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false
export MINIKUBE_HOME=$HOME
export CHANGE_MINIKUBE_NONE_USER=true
export KUBECONFIG=$HOME/.kube/config
export K8S_VERSION=1.14.0
export MINIKUBE_VERSION=1.1.1

echo "Install kubectl ..."
# Travis CI VMs allow sudo without password.
K8S_RELEASE_SITE="https://storage.googleapis.com/kubernetes-release/release"
sudo curl -sLo /usr/local/bin/kubectl \
     $K8S_RELEASE_SITE/v$K8S_VERSION/bin/linux/amd64/kubectl
sudo chmod a+x /usr/local/bin/kubectl

echo "Install minikube ..."
MINIKUBE_RELEASE_SITE="https://storage.googleapis.com/minikube/releases"
sudo curl -sLo /usr/local/bin/minikube \
     $MINIKUBE_RELEASE_SITE/v$MINIKUBE_VERSION/minikube-linux-amd64
sudo chmod a+x /usr/local/bin/minikube

echo "Configure minikube ..."
mkdir -p $HOME/.kube $HOME/.minikube
touch $KUBECONFIG

echo "Start minikube cluster ..."
sudo minikube start \
     --vm-driver=none \
     --kubernetes-version=v$K8S_VERSION \
     --cpus 2 \
     --memory 6144
sudo chown -R travis: $HOME/.minikube/
kubectl cluster-info

echo "Install Argo on minikube cluster ..."
kubectl create namespace argo
kubectl apply -n argo -f \
  https://raw.githubusercontent.com/argoproj/argo/stable/manifests/install.yaml
kubectl create rolebinding default-admin \
  --clusterrole=admin \
  --serviceaccount=default:default

echo "Install Tekton on minikube cluster ..."
TEKTON_RELEASE_SITE="https://storage.googleapis.com/tekton-releases/pipeline"
kubectl apply --filename $TEKTON_RELEASE_SITE/previous/v0.10.1/release.yaml
