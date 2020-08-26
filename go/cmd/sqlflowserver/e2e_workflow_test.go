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
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
)

func TestEnd2EndWorkflow(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DATASOURCE") == "" || strings.ToLower(os.Getenv("SQLFLOW_TEST")) != "workflow" {
		t.Skip("Skipping workflow test.")
	}
	driverName, _, err := database.ParseURL(testDatasource)
	a.NoError(err)

	if driverName != "mysql" && driverName != "maxcompute" && driverName != "alisa" {
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

	if driverName == "maxcompute" {
		AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
		SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
		endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
		dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
		caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
		if caseDB == "" {
			t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
		}
		caseTrainTable = caseDB + ".sqlflow_test_iris_train"
		caseTestTable = caseDB + ".sqlflow_test_iris_test"
		casePredictTable = caseDB + ".sqlflow_test_iris_predict"
		// write model to current MaxCompute project
		caseInto = "my_dnn_model"
	} else if driverName == "alisa" {
		dbConnStr = os.Getenv("SQLFLOW_DATASOURCE")
		caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
		caseTrainTable = caseDB + ".sqlflow_test_iris_train"
		caseTestTable = caseDB + ".sqlflow_test_iris_test"
		casePredictTable = caseDB + ".sqlflow_test_iris_predict"
	}

	t.Run("CaseWorkflowTrainAndPredictDNNCustomImage", CaseWorkflowTrainAndPredictDNNCustomImage)
	t.Run("CaseWorkflowTrainAndPredictDNN", CaseWorkflowTrainAndPredictDNN)
	t.Run("CaseTrainDistributedPAIArgo", CaseTrainDistributedPAIArgo)
	t.Run("CaseBackticksInSQL", CaseBackticksInSQL)
	t.Run("CaseWorkflowStepErrorMessage", CaseWorkflowStepErrorMessage)
	// test experimental workflow generation
	os.Setenv("SQLFLOW_WORKFLOW_BACKEND", "experimental")
	t.Run("CaseWorkflowTrainXgboost", CaseWorkflowTrainXgboost)
	os.Setenv("SQLFLOW_WORKFLOW_BACKEND", "")
}

func CaseWorkflowStepErrorMessage(t *testing.T) {
	a := assert.New(t)
	sqlProgram := fmt.Sprintf(`
SELECT *
FROM %s
TO TRAIN DNNClassifier
WITH
	model.no_exists_param = 3,
	model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;	
	`, caseTrainTable, caseInto)
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
	e := checkWorkflow(ctx, cli, stream)
	a.Error(e)
	a.Contains(e.Error(), "runSQLProgram error: unsupported attribute model.no_exists_param")
}

func CaseWorkflowTrainAndPredictDNN(t *testing.T) {
	a := assert.New(t)

	sqlProgram := fmt.Sprintf(`
SELECT * FROM %s LIMIT 10;

SELECT * FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM %s"
LABEL class
INTO %s;

SELECT * FROM %s
TO EVALUATE %s
WITH validation.metrics="Accuracy"
LABEL class
INTO %s.sqlflow_iris_eval_result;

SELECT * FROM %s
TO PREDICT %s.class
USING %s;

SELECT *
FROM %s LIMIT 5;
	`, caseTrainTable, caseTrainTable, caseTestTable, caseInto,
		caseTestTable, caseInto, caseDB,
		caseTestTable, casePredictTable, caseInto, casePredictTable)

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

func CaseWorkflowTrainAndPredictDNNCustomImage(t *testing.T) {
	if os.Getenv("SQLFLOW_submitter") != "pai" && os.Getenv("SQLFLOW_submitter") != "alisa" {
		t.Skip("Skip PAI case.")
	}
	a := assert.New(t)
	// use the default image to test
	customImage := os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE")
	sqlProgram := fmt.Sprintf(`
SELECT * FROM %s LIMIT 10;

SELECT * FROM %s
TO TRAIN %s/DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [64, 32],
	validation.select = "SELECT * FROM %s"
LABEL class
INTO test_workflow_model;`, caseTrainTable, caseTrainTable, customImage, caseTestTable)

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

func checkWorkflow(ctx context.Context, cli pb.SQLFlowClient, stream pb.SQLFlow_RunClient) error {
	var workflowID string
	for {
		iter, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream read err: %v", err)
		}
		workflowID = iter.GetJob().GetId()
	}
	if !strings.HasPrefix(workflowID, "sqlflow") {
		return fmt.Errorf("workflow ID: %s does not prefix with sqlflow", workflowID)
	}
	req := &pb.FetchRequest{
		Job: &pb.Job{Id: workflowID},
	}
	// wait 30min for the workflow execution since it may take time to allocate enough nodes.
	// each loop waits 3 seconds, total 600 * 3 = 1800 seconds
	for i := 0; i < 600; i++ {
		res, err := cli.Fetch(ctx, req)
		if err != nil {
			return err
		}
		if res.Eof {
			// pass the test case
			return nil
		}
		req = res.UpdatedFetchSince
		time.Sleep(4 * time.Second)
	}
	return fmt.Errorf("workflow times out")
}

func CaseTrainDistributedPAIArgo(t *testing.T) {
	if os.Getenv("SQLFLOW_submitter") != "pai" && os.Getenv("SQLFLOW_submitter") != "alisa" {
		t.Skip("Skip PAI case.")
	}
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
	SELECT * FROM %s
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		train.num_workers=2,
		train.num_ps=2,
		train.save_checkpoints_steps=20,
		train.epoch=2,
		train.batch_size=4,
		train.verbose=2,
		validation.select="select * from %s"
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;

	SELECT * FROM %s TO PREDICT %s.class USING %s;
	`, caseTrainTable, caseTestTable, caseInto, caseTestTable, casePredictTable, caseInto)

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	// wait 1h for the workflow execution since it may take time to allocate enough nodes.
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL, Session: &pb.Session{DbConnStr: testDatasource}})
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	a.NoError(checkWorkflow(ctx, cli, stream))
}

func CaseWorkflowRunBinary(t *testing.T) {
	a := assert.New(t)
	runSQL := fmt.Sprintf(`
	SELECT * FROM %s
	TO RUN sqlflow/sqlflow:step
	CMD "echo", "Hello World"
	`, caseTrainTable)

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	// wait 1h for the workflow execution since it may take time to allocate enough nodes.
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: runSQL, Session: &pb.Session{DbConnStr: testDatasource}})
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	a.NoError(checkWorkflow(ctx, cli, stream))
}

func CaseBackticksInSQL(t *testing.T) {
	driverName, _, _ := database.ParseURL(testDatasource)
	if driverName != "mysql" {
		t.Skip("Skipping workflow mysql test.")
	}

	a := assert.New(t)
	trainSQL := fmt.Sprintf("SELECT `sepal_length`, `class` FROM %s"+`
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select="select * from %s"
	LABEL class
	INTO %s;`, caseTrainTable, caseTestTable, caseInto)

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	// wait 1h for the workflow execution since it may take time to allocate enough nodes.
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL, Session: &pb.Session{DbConnStr: testDatasource}})
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	a.NoError(checkWorkflow(ctx, cli, stream))
}

func TestEnd2EndFluidWorkflow(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DATASOURCE") == "" || strings.ToLower(os.Getenv("SQLFLOW_TEST")) != "workflow" {
		t.Skip("Skipping workflow test.")
	}
	driverName, _, err := database.ParseURL(testDatasource)
	a.NoError(err)

	if driverName != "mysql" && driverName != "maxcompute" && driverName != "alisa" {
		t.Skip("Skipping workflow test.")
	}
	modelDir := ""
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	//TODO(yancey1989): using the same end-to-end workflow test with the Couler backend
	os.Setenv("SQLFLOW_WORKFLOW_BACKEND", "fluid")
	go start(modelDir, caCrt, caKey, unitTestPort, true)
	server.WaitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("CaseWorkflowTrainAndPredictDNN", CaseWorkflowTrainAndPredictDNN)
}

func CaseWorkflowTrainXgboost(t *testing.T) {
	a := assert.New(t)

	testMain := func(sqlProgram string) {
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

	extraTrainSQLProgram := `SELECT * FROM iris.train LIMIT 100;

SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH objective="multi:softmax",num_class=3
LABEL class
INTO sqlflow_models.xgb_classification;

SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH objective="multi:softmax",num_class=3
COLUMN sepal_length, DENSE(sepal_width)
LABEL class
INTO sqlflow_models.xgb_classification;

SELECT * FROM sqlflow_models.xgb_classification;
`

	sqlProgram := `
SELECT * FROM iris.test
TO PREDICT iris.test_result_table.class
USING sqlflow_models.xgb_classification;

SELECT * FROM iris.test_result_table;

SELECT * FROM iris.test
TO EVALUATE sqlflow_models.xgb_classification
WITH
	validation.metrics="accuracy_score"
LABEL class
INTO iris.evaluate_result_table;

SELECT * FROM iris.evaluate_result_table;
`
	testMain(extraTrainSQLProgram + sqlProgram)
	testMain(sqlProgram)
}
