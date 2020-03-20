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
function test_fluid() {
  kubectl delete task echo-hello-world --ignore-not-found=true
  kubectl delete taskrun echo-hello-world-run --ignore-not-found=true
  cat <<EOF > /tmp/fluid_demo.py
import fluid

@fluid.task
def echo_hello_world(hello, world="El mundo"):
    fluid.step(image="docker/whalesay", cmd=["echo"], args=[hello])

echo_hello_world("Aloha")
EOF
  python /tmp/fluid_demo.py > /tmp/fluid_demo.yaml
  MESSAGE=$(kubectl create -f /tmp/fluid_demo.yaml)
  TASKRUN_NAME=$(echo ${MESSAGE} | cut -d ' ' -f 1 | cut -d '/' -f 2)-run
  echo TaskRun: ${TASKRUN_NAME}
  for i in {1..30}; do
    TASKRUN_TYPE=$(kubectl get taskrun ${TASKRUN_NAME} -o jsonpath='{.status.conditions[0].type}')
    TASKRUN_STATUS=$(kubectl get taskrun ${TASKRUN_NAME} -o jsonpath='{.status.conditions[0].status}')
    if [ "$TASKRUN_TYPE" == "Succeeded" ] && [ "$TASKRUN_STATUS" == "True" ]; then
      echo "Tekton TaskRun Succeed"
      return 0
    else
      echo "Wait Tekton TaskRun Succeed for 5s..."
      sleep 5
    fi
  done
  echo "Wait Tekton TaskRun succeed timeout."
  exit 1
}

test_fluid
