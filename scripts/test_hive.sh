#!/bin/bash
set -e
. /miniconda/etc/profile.d/conda.sh
source activate sqlflow-dev

# wait until hive test server is ready
while [ 1 ]
do
nc -z localhost 10000
if [ $? -eq 0 ]; then
break
else
echo "port not ready"
sleep 1
fi
done

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_TEST_DB=hive SQLFLOW_log_level=debug go test -v ./...
