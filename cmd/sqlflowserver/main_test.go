// Copyright 2019 The SQLFlow Authors. All rights reserved.
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
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"github.com/sql-machine-learning/sqlflow/sql"
	"github.com/sql-machine-learning/sqlflow/sql/testdata"
	"github.com/stretchr/testify/assert"
)

var dbConnStr string

var caseDB = "iris"
var caseTrainTable = "train"
var caseTestTable = "test"
var casePredictTable = "predict"

func WaitPortReady(addr string, timeout time.Duration) {
	// Set default timeout to
	if timeout == 0 {
		timeout = time.Duration(1) * time.Second
	}
	for {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			log.Printf("%s, try again", err.Error())
		}
		if conn != nil {
			err = conn.Close()
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func sqlRequest(sql string) *pb.Request {
	se := &pb.Session{Token: "user-unittest", DbConnStr: dbConnStr}
	return &pb.Request{Sql: sql, Session: se}
}

func AssertEqualAny(a *assert.Assertions, expected interface{}, actual *any.Any) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.StringValue":
		b := wrappers.StringValue{}
		ptypes.UnmarshalAny(actual, &b)
		a.Equal(expected, b.Value)
	case "type.googleapis.com/google.protobuf.FloatValue":
		b := wrappers.FloatValue{}
		ptypes.UnmarshalAny(actual, &b)
		a.Equal(float32(expected.(float64)), b.Value)
	case "type.googleapis.com/google.protobuf.DoubleValue":
		b := wrappers.DoubleValue{}
		ptypes.UnmarshalAny(actual, &b)
		a.Equal(expected.(float64), b.Value)
	case "type.googleapis.com/google.protobuf.Int64Value":
		b := wrappers.Int64Value{}
		ptypes.UnmarshalAny(actual, &b)
		a.Equal(expected.(int64), b.Value)
	}
}

func AssertGreaterEqualAny(a *assert.Assertions, actual *any.Any, expected interface{}) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.Int64Value":
		b := wrappers.Int64Value{}
		ptypes.UnmarshalAny(actual, &b)
		a.GreaterOrEqual(b.Value, expected.(int64))
	}
}

func AssertContainsAny(a *assert.Assertions, all map[string]string, actual *any.Any) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.StringValue":
		b := wrappers.StringValue{}
		ptypes.UnmarshalAny(actual, &b)
		if _, ok := all[b.Value]; !ok {
			a.Failf("string value %s not exist", b.Value)
		}
	}
}

func ParseRow(stream pb.SQLFlow_RunClient) ([]string, [][]*any.Any) {
	var rows [][]*any.Any
	var columns []string
	counter := 0
	for {
		iter, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("stream read err: %v", err)
		}
		if counter == 0 {
			head := iter.GetHead()
			columns = head.GetColumnNames()
		} else {
			onerow := iter.GetRow().GetData()
			rows = append(rows, onerow)
		}
		counter++
	}
	return columns, rows
}

func prepareTestData(dbStr string) error {
	// popularize test data
	testDB, err := sql.NewDB(dbStr)
	if err != nil {
		return err
	}
	if os.Getenv("SQLFLOW_TEST_DB") != "maxcompute" {
		_, err = testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
		if err != nil {
			return err
		}
	}

	switch os.Getenv("SQLFLOW_TEST_DB") {
	case "mysql":
		if err := testdata.Popularize(testDB.DB, testdata.IrisSQL); err != nil {
			return err
		}
		if err := testdata.Popularize(testDB.DB, testdata.ChurnSQL); err != nil {
			return err
		}
		return testdata.Popularize(testDB.DB, testdata.TextCNSQL)
	case "hive":
		if err := testdata.Popularize(testDB.DB, testdata.IrisHiveSQL); err != nil {
			return err
		}
		return testdata.Popularize(testDB.DB, testdata.ChurnHiveSQL)
	case "maxcompute":
		if err := testdata.Popularize(testDB.DB, testdata.IrisMaxComputeSQL); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("unrecognized SQLFLOW_TEST_DB %s", os.Getenv("SQLFLOW_TEST_DB"))
}

func generateTempCA() (tmpDir, caCrt, caKey string, err error) {
	tmpDir, _ = ioutil.TempDir("/tmp", "sqlflow_ssl_")
	caKey = path.Join(tmpDir, "ca.key")
	caCsr := path.Join(tmpDir, "ca.csr")
	caCrt = path.Join(tmpDir, "ca.crt")
	if err = exec.Command("openssl", "genrsa", "-out", caKey, "2048").Run(); err != nil {
		return
	}
	if err = exec.Command("openssl", "req", "-nodes", "-new", "-key", caKey, "-subj", "/CN=localhost", "-out", caCsr).Run(); err != nil {
		return
	}
	if err = exec.Command("openssl", "x509", "-req", "-sha256", "-days", "365", "-in", caCsr, "-signkey", caKey, "-out", caCrt).Run(); err != nil {
		return
	}
	os.Setenv("SQLFLOW_CA_CRT", caCrt)
	os.Setenv("SQLFLOW_CA_KEY", caKey)
	return
}

func createRPCConn() (*grpc.ClientConn, error) {
	caCrt := os.Getenv("SQLFLOW_CA_CRT")
	if caCrt != "" {
		creds, _ := credentials.NewClientTLSFromFile(caCrt, "localhost")
		return grpc.Dial("localhost"+port, grpc.WithTransportCredentials(creds))
	}
	return grpc.Dial("localhost"+port, grpc.WithInsecure())
}

func TestEnd2EndMySQL(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	// default run mysql tests
	if len(testDBDriver) == 0 {
		testDBDriver = "mysql"
	}
	if testDBDriver != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	dbConnStr = "mysql://root:root@tcp/?maxAllowedPacket=0"
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	go start("", modelDir, caCrt, caKey, true)
	WaitPortReady("localhost"+port, 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
	t.Run("TestTextClassification", CaseTrainTextClassification)
	t.Run("CaseTrainTextClassificationCustomLSTM", CaseTrainTextClassificationCustomLSTM)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainSQLWithHyperParams", CaseTrainSQLWithHyperParams)
	t.Run("CaseTrainCustomModelWithHyperParams", CaseTrainCustomModelWithHyperParams)
}

func TestEnd2EndHive(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir := ""
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	if testDBDriver != "hive" {
		t.Skip("Skipping hive tests")
	}
	dbConnStr = "hive://127.0.0.1:10000/iris"
	go start("", modelDir, caCrt, caKey, true)
	WaitPortReady("localhost"+port, 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func TestEnd2EndMaxCompute(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
	defer os.RemoveAll(modelDir)
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("MAXCOMPUTE_AK")
	SK := os.Getenv("MAXCOMPUTE_SK")
	endpoint := os.Getenv("MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	go start("", modelDir, caCrt, caKey, true)
	WaitPortReady("localhost"+port, 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	caseDB = "gomaxcompute_driver_w7u"
	caseTrainTable = "sqlflow_test_iris_train"
	caseTestTable = "sqlflow_test_iris_test"
	casePredictTable = "sqlflow_test_iris_predict"
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func CaseShowDatabases(t *testing.T) {
	a := assert.New(t)
	cmd := "show databases;"
	conn, err := createRPCConn()

	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(cmd))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	head, resp := ParseRow(stream)
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		a.Equal("database_name", head[0])
	} else {
		a.Equal("Database", head[0])
	}

	expectedDBs := map[string]string{
		"information_schema": "",
		"churn":              "",
		"iris":               "",
		"mysql":              "",
		"performance_schema": "",
		"sqlflow_models":     "",
		"sqlfs_test":         "",
		"sys":                "",
		"text_cn":            "",
		"hive":               "", // if current mysql is also used for hive
		"default":            "", // if fetching default hive databases
	}
	for i := 0; i < len(resp); i++ {
		AssertContainsAny(a, expectedDBs, resp[i][0])
	}
}

func CaseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf("select * from %s.%s limit 2;", caseDB, caseTrainTable)
	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(cmd))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	head, rows := ParseRow(stream)
	expectedHeads := []string{
		"sepal_length",
		"sepal_width",
		"petal_length",
		"petal_width",
		"class",
	}
	for idx, headCell := range head {
		if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
			a.Equal("train."+expectedHeads[idx], headCell)
		} else {
			a.Equal(expectedHeads[idx], headCell)
		}
	}
	expectedRows := [][]interface{}{
		{6.4, 2.8, 5.6, 2.2, int64(2)},
		{5.0, 2.3, 3.3, 1.0, int64(1)},
	}
	for rowIdx, row := range rows {
		for colIdx, rowCell := range row {
			AssertEqualAny(a, expectedRows[rowIdx][colIdx], rowCell)
		}
	}
}

// CaseTrainSQL is a simple End-to-End testing for case training and predicting
func CaseTrainSQL(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT *
FROM %s.%s
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`, caseDB, caseTrainTable)

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)

	predSQL := fmt.Sprintf(`SELECT *
FROM %s.%s
PREDICT %s.%s.class
USING sqlflow_models.my_dnn_model;`, caseDB, caseTestTable, caseDB, casePredictTable)

	stream, err = cli.Run(ctx, sqlRequest(predSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait predict finish
	ParseRow(stream)

	showPred := fmt.Sprintf(`SELECT *
FROM %s.%sLIMIT 5;`, caseDB, casePredictTable)

	stream, err = cli.Run(ctx, sqlRequest(showPred))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	_, rows := ParseRow(stream)

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[4], int64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 4 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 4)
	}
}

// CaseTrainCustomModel tests using customized models
func CaseTrainCustomModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN sqlflow_models.DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)

	predSQL := `SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model_custom;`

	stream, err = cli.Run(ctx, sqlRequest(predSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait predict finish
	ParseRow(stream)

	showPred := `SELECT *
FROM iris.predict LIMIT 5;`

	stream, err = cli.Run(ctx, sqlRequest(showPred))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	_, rows := ParseRow(stream)

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[4], int64(0))
	}
}

// CaseTrainTextClassification is a simple End-to-End testing for case training
// text classification models.
func CaseTrainTextClassification(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN DNNClassifier
WITH n_classes = 17, hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)
}

// CaseTrainTextClassificationCustomLSTM is a simple End-to-End testing for case training
// text classification models.
func CaseTrainTextClassificationCustomLSTM(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN sqlflow_models.StackedBiLSTMClassifier
WITH n_classes = 17, stack_units = [16], EPOCHS = 1, BATCHSIZE = 32
COLUMN EMBEDDING(SEQ_CATEGORY_ID(news_title,1600,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_bilstm_model;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)
}

func CaseTrainSQLWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20], BATCHSIZE = 10, EPOCHS = 2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)
}

// CaseTrainCustomModel tests using customized models
func CaseTrainCustomModelWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN sqlflow_models.DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20], BATCHSIZE = 10, EPOCHS=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`

	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainSQL))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)
}
