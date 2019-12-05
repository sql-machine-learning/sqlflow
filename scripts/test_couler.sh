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

############# Run Couler unit tests #############
pip install -r python/couler/requirements.txt

pytest python/couler/tests


############# Run Couler e2e test #############
CHECK_INTERVAL_SECS=2

function test_couler() {

    cat <<EOF > /tmp/sqlflow_couler.py
import couler.argo as couler
couler.run_container(image="docker/whalesay", command='echo "SQLFlow bridges AI and SQL engine."')
EOF


    couler run --mode argo --file /tmp/sqlflow_couler.py > /tmp/sqlflow_argo.yaml

    MESSAGE=$(kubectl create -f /tmp/sqlflow_argo.yaml)

    WORKFLOW_NAME=$(echo ${MESSAGE} | cut -d ' ' -f 1 | cut -d '/' -f 2)

    echo WORKFLOW_NAME ${WORKFLOW_NAME}

    for i in {1..30}; do
        WORKFLOW_STATUS=$(kubectl get wf ${WORKFLOW_NAME} -o jsonpath='{.status.phase}')

        if [[ "$WORKFLOW_STATUS" == "Succeeded" ]]; then
            echo "Argo workflow succeeded."
            kubectl delete wf ${WORKFLOW_NAME}
            rm -rf /tmp/sqlflow* 
            return 0
        else
            echo "Argo workflow ${WORKFLOW_NAME} ${WORKFLOW_STATUS}"
            sleep ${CHECK_INTERVAL_SECS}
        fi
    done
    return 1
}

test_couler
ret=$?

if [[ "$ret" != "0" ]]; then
    echo "Argo job timed out."
    rm -rf /tmp/sqlflow* 
    exit 1
fi

############# Run SQLFLow test with Argo Mode #############
set -x
set -e
# start a SQLFlow MySQL Pod with testdata
kubectl run mysql --port 3306 --env="SQLFLOW_MYSQL_HOST=0.0.0.0" --env="SQLFLOW_MYSQL_PORT=3306" --image=sqlflow/sqlflow --command -- bash /start.sh mysql
MYSQL_POD_NAME=$(kubectl get pod -l run=mysql -o jsonpath="{.items[0].metadata.name}")
for i in {1..30}
do
    MYSQL_POD_STATUS=$(kubectl get pod ${MYSQL_POD_NAME} -o jsonpath='{.status.phase}')
    echo ${MYSQL_POD_STATUS}
    if [[ "${MYSQL_POD_STATUS}" == "Running" ]]; then
        echo "SQLFlow MySQL Pod running."
        MYSQL_POD_IP=$(kubectl get pod ${MYSQL_POD_NAME} -o jsonpath='{.status.podIP}')
        go generate ./...
        SQLFLOW_TEST_DATASOURCE="mysql://root:root@tcp(${MYSQL_POD_IP}:3306)/?maxAllowedPacket=0" SQLFLOW_ARGO_MODE=True go test ./cmd/... -run TestEnd2EndMySQLWorkflow -v
        exit 0
    else
        echo "Wait SQLFlow MySQL Pod ${MYSQL_POD_NAME}"
        sleep ${CHECK_INTERVAL_SECS}
    fi
done

echo "Start SQLFlow MySQL Pod failed."
exit 1