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

changed_fileext=$(git diff --name-only HEAD..origin/develop --|awk -F. '{print $NF}'|uniq)
if [[ "$changed_fileext" == "md" ]]; then
    echo "Only Markdown files changed.  No need to run unit tests."
    exit 0
fi

docker pull docker/whalesay

export SQLFLOW_TEST=workflow
export SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT=http://localhost:8001

echo "Run Couler unit tests ..."
pip -q install -r python/couler/requirements.txt
pytest --cov=./ python/couler/tests


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


# shellcheck disable=SC2154
if [[ "$SQLFLOW_submitter" == "pai" ]]; then
    echo "Test submitting PAI job using Argo workflow mode ..."
    export SQLFLOW_TEST_DATASOURCE="maxcompute://${MAXCOMPUTE_AK}:${MAXCOMPUTE_SK}@${MAXCOMPUTE_ENDPOINT}"
    gotest -p 1 -covermode=count -coverprofile=profile.out -run TestEnd2EndWorkflow -v ./go/cmd/...
    if [ -f profile.out ]; then
        cat profile.out > coverage.txt
        rm profile.out
    fi
    echo "Run unit tests of go/workflow/argo ..."
    gotest -p 1 -covermode=count -coverprofile=profile.out -v ./go/workflow/argo/
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
else
    echo "Create a MySQL pod on Kubernetes ..."
    kubectl delete po mysql || true
    kubectl create -f ./scripts/test/mysql_pod.yaml

    TIMEOUT="true"
    for _ in {1..30}; do
        MYSQL_POD_READY=$(kubectl get pod mysql -o jsonpath='{.status.containerStatuses[0].ready}')
        echo "Check MySQL Pod is ready ..." "${MYSQL_POD_READY}"
        if [[ "${MYSQL_POD_READY}" == "true" ]]; then
            MYSQL_POD_IP=$(kubectl get pod mysql -o jsonpath='{.status.podIP}')
            echo "MySQL pod IP: $MYSQL_POD_IP"
            export SQLFLOW_TEST_DB="mysql"
            export SQLFLOW_TEST_DATASOURCE="mysql://root:root@tcp(${MYSQL_POD_IP}:3306)/?maxAllowedPacket=0"

            go generate ./...
            # Refer to https://github.com/codecov/example-go for merging coverage
            # from multiple runs of tests.
            gotest -p 1 -covermode=count -coverprofile=profile.out -v \
                -run TestEnd2EndWorkflow -timeout 2400s ./go/cmd/...
            if [ -f profile.out ]; then
                cat profile.out > coverage.txt
                rm profile.out
            fi
            gotest -p 1 -covermode=count -coverprofile=profile.out -v \
                ./go/workflow/argo/...
            if [ -f profile.out ]; then
                cat profile.out >> coverage.txt
                rm profile.out
            fi

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
fi
