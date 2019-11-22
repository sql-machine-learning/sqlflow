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

CHECK_INTERVAL_SECS=2
MESSAGE=$(kubectl create -f https://raw.githubusercontent.com/argoproj/argo/master/examples/hello-world.yaml)
WORKFLOW_NAME=$(echo ${MESSAGE} | cut -d ' ' -f 1 | cut -d '/' -f 2)

echo WORKFLOW_NAME ${WORKFLOW_NAME}

for i in {1..30}; do
    WORKFLOW_STATUS=$(kubectl get wf ${WORKFLOW_NAME} -o jsonpath='{.status.phase}')

    if [[ "$WORKFLOW_STATUS" == "Succeeded" ]]; then
        echo "Argo workflow succeeded."
        kubectl delete wf ${WORKFLOW_NAME}
        exit 0
    else
        echo "Argo workflow ${WORKFLOW_NAME} ${WORKFLOW_STATUS}"
        sleep ${CHECK_INTERVAL_SECS}
    fi
done

echo "Argo job timed out."
exit 1
