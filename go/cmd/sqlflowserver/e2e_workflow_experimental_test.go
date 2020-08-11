// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func TestEnd2EndWorkflowExperimental(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DATASOURCE") == "" || strings.ToLower(os.Getenv("SQLFLOW_TEST")) != "workflow" {
		t.Skip("Skipping workflow test.")
	}
	os.Setenv("SQLFLOW_WORKFLOW_BACKEND", "experimental")

	driverName, _, err := database.ParseURL(testDatasource)
	a.NoError(err)

	if driverName != "mysql" {
		t.Skip("Skipping workflow test.")
	}
	modelDir := ""
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	go start(modelDir, caCrt, caKey, unitTestPort, true)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	t.Run("CaseWorkflowTrainXgboost", CaseWorkflowTrainXgboost)
	// set back SQLFLOW_WORKFLOW_BACKEND
	os.Setenv("SQLFLOW_WORKFLOW_BACKEND", "")
}

func CaseWorkflowTrainXgboost(t *testing.T) {
	a := assert.New(t)

	sqlProgram := `SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH objective="multi:softmax",num_class=3
LABEL class
INTO sqlflow_models.xgb_classification;`

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: sqlProgram, Session: &pb.Session{DbConnStr: testDatasource}})
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	a.NoError(checkWorkflow(ctx, cli, stream))
}
