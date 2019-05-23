#!/bin/bash

# Wait until hive test server is ready, port 8899
# is a status port indicates the hive server container
# is ready, see .travis.yml for the details
while true; do
  curl http://localhost:8899 2>/dev/null
  if [ $? -eq 0 ]; then
    break
  else
    echo "still waiting, hive server is not ready..."
    sleep 5
  fi
done

set -e
. /miniconda/etc/profile.d/conda.sh
source activate sqlflow-dev

go generate ./...
go get -v -t ./...
go install ./...
SQLFLOW_TEST_DB=hive SQLFLOW_log_level=debug go test -v ./...
