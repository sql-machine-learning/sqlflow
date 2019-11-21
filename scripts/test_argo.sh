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

# Set up Argo
kubectl create namespace argo
kubectl apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/stable/manifests/install.yaml
kubectl create rolebinding default-admin --clusterrole=admin --serviceaccount=default:default

CHECK_INTERVAL_SECS=2
MESSAGE=$(kubectl create -f https://raw.githubusercontent.com/argoproj/argo/master/examples/hello-world.yaml)
POD_NAME=$(echo ${MESSAGE} | cut -d ' ' -f 1 | cut -d '/' -f 2)

echo POD_NAME ${POD_NAME}

for i in {1..30}; do
    JOB_STATUS=$(kubectl get pod ${POD_NAME} -o jsonpath='{.status.phase}')

    if [[ "$JOB_STATUS" == "Succeeded" ]]; then
        echo "Argo job succeeded."
        kubectl delete pod ${POD_NAME}
        exit 0
    else
        echo "Argo job ${POD_NAME} ${JOB_STATUS}"
        sleep ${CHECK_INTERVAL_SECS}
    fi
done

echo "Argo job timed out."
exit 1
