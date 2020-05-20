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

changed_fileext=$(git diff --name-only HEAD..develop|awk -F. '{print $NF}'|uniq)
if [[ "$changed_fileext" == "md" ]]; then
    echo "Only Markdown files changed, skip test."
    exit 0
fi

docker pull docker/whalesay

export SQLFLOW_TEST=workflow
export SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT=http://localhost:8001

echo "Run Couler unit tests ..."
pip -q install -r python/couler/requirements.txt
pytest python/couler/tests


echo "Run Couler end-to-end test ..."
CHECK_INTERVAL_SECS=2

cat <<EOF > /tmp/sqlflow_couler.py
import couler.argo as couler
couler.run_container(
  image="docker/whalesay",
  command='echo "SQLFlow bridges AI and SQL engine."')
EOF

couler run --mode argo --file /tmp/sqlflow_couler.py > /tmp/sqlflow_argo.yaml

MESSAGE=$(kubectl create -f /tmp/sqlflow_argo.yaml)
WORKFLOW_NAME=$(echo "$MESSAGE" | cut -d ' ' -f 1 | cut -d '/' -f 2)
echo "Workflow name: $WORKFLOW_NAME"

TIMEOUT="true"
for _ in {1..30}; do
    STATUS=$(kubectl get wf "${WORKFLOW_NAME}" -o jsonpath='{.status.phase}')
    if [[ "$STATUS" == "Succeeded" ]]; then
        echo "Argo workflow succeeded."
        kubectl delete wf "${WORKFLOW_NAME}"
        rm -rf /tmp/sqlflow*
        TIMEOUT="false"
        break
    else
        sleep "$CHECK_INTERVAL_SECS"
    fi
done

if [[ "$TIMEOUT" == "true" ]]; then
    echo "Workflow job timeout."
    exit 1
fi


echo "Test access MySQL deployed on Kubernetes ..."

# Start a SQLFlow MySQL Pod with testdata
kubectl run mysql --port 3306 \
        --env="MYSQL_HOST=0.0.0.0" \
        --env="MYSQL_PORT=3306" \
        --image="sqlflow:mysql" \
        --command -- bash /start.sh
POD=$(kubectl get pod -l run=mysql -o jsonpath="{.items[0].metadata.name}")

TIMEOUT="true"
for _ in {1..30}; do
    MYSQL_POD_STATUS=$(kubectl get pod "$POD" -o jsonpath='{.status.phase}')
    echo "${MYSQL_POD_STATUS}"
    if [[ "${MYSQL_POD_STATUS}" == "Running" ]]; then
        MYSQL_POD_IP=$(kubectl get pod "$POD" -o jsonpath='{.status.podIP}')
        echo "MySQL pod IP: $MYSQL_POD_IP"
        export SQLFLOW_TEST_DATASOURCE="mysql://root:root@tcp(${MYSQL_POD_IP}:3306)/?maxAllowedPacket=0"
        kubectl logs "$POD"
        go generate ./...
        gotest ./cmd/... -run TestEnd2EndWorkflow -v
        gotest ./pkg/workflow/argo/... -v
        TIMEOUT=false
        break
    else
        sleep ${CHECK_INTERVAL_SECS}
    fi
done

if [[ "$TIMEOUT" == "true" ]]; then
    echo "Launching MySQL pod timeout"
    exit 1
fi


echo "Test submitting PAI job using Argo workflow mode ..."
# shellcheck disable=SC2154
if [[ "$SQLFLOW_submitter" == "pai" ]]; then
    # TDOO(wangkuiyi): rename MAXCOMPUTE_AK to SQLFLOW_TEST_DB_MAXCOMPUTE_ASK
    # later after rename the Travis CI env settings.
    export SQLFLOW_TEST_DATASOURCE="maxcompute://${MAXCOMPUTE_AK}:${MAXCOMPUTE_SK}@${MAXCOMPUTE_ENDPOINT}"
    gotest ./cmd/... -run TestEnd2EndWorkflow -v
fi


echo "Run unit tests of pkg/workflow/argo ..."
gotest -v ./pkg/workflow/argo/


# TODO(yancey1989): run fluid test if tekton on SQLFlow it's ready.
# bash ./scripts/test/fluid.sh
# gotest ./cmd/... -run TestEnd2EndFluidWorkflow -v
