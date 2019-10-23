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
	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/sql/testdata"
)

var dbConnStr string

var caseDB = "iris"
var caseTrainTable = "train"
var caseTestTable = "test"
var casePredictTable = "predict"

const unitestPort = 50051

func serverIsReady(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	err = conn.Close()
	return err == nil
}

func waitPortReady(addr string, timeout time.Duration) {
	// Set default timeout to
	if timeout == 0 {
		timeout = time.Duration(1) * time.Second
	}
	for !serverIsReady(addr, timeout) {
		time.Sleep(1 * time.Second)
	}
}

func connectAndRunSQL(sql string) ([]string, [][]*any.Any, error) {
	conn, err := createRPCConn()
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	stream, err := cli.Run(ctx, sqlRequest(sql))
	if err != nil {
		return nil, nil, err
	}
	cols, rows := ParseRow(stream)
	return cols, rows, nil
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
	case "type.googleapis.com/google.protobuf.FloatValue":
		b := wrappers.FloatValue{}
		ptypes.UnmarshalAny(actual, &b)
		a.GreaterOrEqual(float32(expected.(float64)), b.Value)
	}
}

func AssertContainsAny(a *assert.Assertions, all map[string]string, actual *any.Any) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.StringValue":
		b := wrappers.StringValue{}
		ptypes.UnmarshalAny(actual, &b)
		if _, ok := all[b.Value]; !ok {
			a.Failf("", "string value %s not exist", b.Value)
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
		if err := testdata.Popularize(testDB.DB, testdata.StandardJoinTest); err != nil {
			return err
		}
		if err := testdata.Popularize(testDB.DB, testdata.HousingSQL); err != nil {
			return err
		}
		return testdata.Popularize(testDB.DB, testdata.TextCNSQL)
	case "hive":
		if err := testdata.Popularize(testDB.DB, testdata.IrisHiveSQL); err != nil {
			return err
		}
		return testdata.Popularize(testDB.DB, testdata.ChurnHiveSQL)
	case "maxcompute":
		submitter := os.Getenv("SQLFLOW_submitter")
		if submitter == "alps" {
			if err := testdata.Popularize(testDB.DB, testdata.ODPSFeatureMapSQL); err != nil {
				return err
			}
			if err := testdata.Popularize(testDB.DB, testdata.ODPSSparseColumnSQL); err != nil {
				return err
			}
			return nil
		}
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
	if output, err := exec.Command("openssl", "genrsa", "-out", caKey, "2048").CombinedOutput(); err != nil {
		err = fmt.Errorf("\n%s\n%s", output, err.Error())
		return "", "", "", err
	}
	if output, err := exec.Command("openssl", "req", "-nodes", "-new", "-key", caKey, "-subj", "/CN=localhost", "-out", caCsr).CombinedOutput(); err != nil {
		err = fmt.Errorf("\n%s\n%s", output, err.Error())
		return "", "", "", err
	}
	if output, err := exec.Command("openssl", "x509", "-req", "-sha256", "-days", "365", "-in", caCsr, "-signkey", caKey, "-out", caCrt).CombinedOutput(); err != nil {
		err = fmt.Errorf("\n%s\n%s", output, err.Error())
		return "", "", "", err
	}
	os.Setenv("SQLFLOW_CA_CRT", caCrt)
	os.Setenv("SQLFLOW_CA_KEY", caKey)
	return
}

func createRPCConn() (*grpc.ClientConn, error) {
	caCrt := os.Getenv("SQLFLOW_CA_CRT")
	if caCrt != "" {
		creds, _ := credentials.NewClientTLSFromFile(caCrt, "localhost")
		return grpc.Dial(fmt.Sprintf("localhost:%d", unitestPort), grpc.WithTransportCredentials(creds))
	}
	return grpc.Dial(fmt.Sprintf("localhost:%d", unitestPort), grpc.WithInsecure())
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
	dbConnStr = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	go start("", modelDir, caCrt, caKey, true, unitestPort)
	waitPortReady(fmt.Sprintf("localhost:%d", unitestPort), 0)
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
	t.Run("CaseSparseFeature", CaseSparseFeature)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", CaseTrainRegression)
	t.Run("CaseTrainXGBoostRegression", CaseTrainXGBoostRegression)
	t.Run("CasePredictXGBoostRegression", CasePredictXGBoostRegression)
	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
}

func TestEnd2EndMySQLIR(t *testing.T) {
	if os.Getenv("SQLFLOW_codegen") != "ir" {
		t.Skip("Skipping ir test")
	}
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	// default run mysql tests
	if len(testDBDriver) == 0 {
		testDBDriver = "mysql"
	}
	if testDBDriver != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	dbConnStr = "mysql://root:root@tcp(localhost:3306)/?maxAllowedPacket=0"
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	addr := fmt.Sprintf("localhost:%d", unitestPort)
	if !serverIsReady(addr, 0) {
		go start("", modelDir, caCrt, caKey, true, unitestPort)
		waitPortReady(addr, 0)
	}
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("CaseTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainTextClassificationIR", CaseTrainTextClassificationIR)
	t.Run("CaseTrainTextClassificationFeatureDerivation", CaseTrainTextClassificationFeatureDerivation)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainSQLWithHyperParams", CaseTrainSQLWithHyperParams)
	t.Run("CaseTrainCustomModelWithHyperParams", CaseTrainCustomModelWithHyperParams)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", CaseTrainRegression)
	t.Run("CaseTrainXGBoostRegressionIR", CaseTrainXGBoostRegression)
	t.Run("CasePredictXGBoostRegressionIR", CasePredictXGBoostRegression)
}

func CaseTrainTextClassificationIR(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(SPARSE(news_title,16000,COMMA), 16000),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
}

func CaseTrainTextClassificationFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
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
	dbConnStr = "hive://127.0.0.1:10000/iris?auth=NOSASL"
	go start("", modelDir, caCrt, caKey, true, unitestPort)
	waitPortReady(fmt.Sprintf("localhost:%d", unitestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
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
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter == "alps" || submitter == "elasticdl" {
		t.Skip("Skip this test case, it's for maxcompute + submitters other than alps and elasticdl.")
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("MAXCOMPUTE_AK")
	SK := os.Getenv("MAXCOMPUTE_SK")
	endpoint := os.Getenv("MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	go start("", modelDir, caCrt, caKey, true, unitestPort)
	waitPortReady(fmt.Sprintf("localhost:%d", unitestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	caseDB = os.Getenv("MAXCOMPUTE_PROJECT")
	caseTrainTable = "sqlflow_test_iris_train"
	caseTestTable = "sqlflow_test_iris_test"
	casePredictTable = "sqlflow_test_iris_predict"
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func TestEnd2EndMaxComputeALPS(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
	defer os.RemoveAll(modelDir)
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter != "alps" {
		t.Skip("Skip, this test is for maxcompute + alps")
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("MAXCOMPUTE_AK")
	SK := os.Getenv("MAXCOMPUTE_SK")
	endpoint := os.Getenv("MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)

	caseDB = os.Getenv("MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env MAXCOMPUTE_PROJECT when testing ALPS cases (SQLFLOW_submitter=alps)!!")
	}
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	go start("", modelDir, caCrt, caKey, true, unitestPort)
	waitPortReady(fmt.Sprintf("localhost:%d", unitestPort), 0)

	t.Run("CaseTrainALPS", CaseTrainALPS)
	t.Run("CaseTrainALPSFeatureMap", CaseTrainALPSFeatureMap)
	t.Run("CaseTrainALPSRemoteModel", CaseTrainALPSRemoteModel)
}

func TestEnd2EndMaxComputeElasticDL(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
	defer os.RemoveAll(modelDir)
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter != "elasticdl" {
		t.Skip("Skip, this test is for maxcompute + ElasticDL")
	}

	if testDBDriver != "maxcompute" {
		t.Skip("Skip maxcompute tests")
	}
	AK := os.Getenv("MAXCOMPUTE_AK")
	SK := os.Getenv("MAXCOMPUTE_SK")
	endpoint := os.Getenv("MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)

	caseDB = os.Getenv("MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env MAXCOMPUTE_PROJECT when testing ElasticDL cases (SQLFLOW_submitter=elasticdl)!!")
	}
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	go start("", modelDir, caCrt, caKey, true, unitestPort)
	waitPortReady(fmt.Sprintf("localhost:%d", unitestPort), 0)

	t.Run("CaseTrainElasticDL", CaseTrainElasticDL)
}

func CaseShowDatabases(t *testing.T) {
	a := assert.New(t)
	cmd := "show databases;"
	head, resp, err := connectAndRunSQL(cmd)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		a.Equal("database_name", head[0])
	} else {
		a.Equal("Database", head[0])
	}

	expectedDBs := map[string]string{
		"information_schema":      "",
		"boston":                  "",
		"churn":                   "",
		"creditcard":              "",
		"feature_derivation_case": "",
		"housing":                 "",
		"iris":                    "",
		"mysql":                   "",
		"performance_schema":      "",
		"sqlflow_models":          "",
		"sf_home":                 "", // default auto train&val database
		"sqlfs_test":              "",
		"sys":                     "",
		"text_cn":                 "",
		"standard_join_test":      "",
		"iris_e2e":                "", // created by Python e2e test
		"hive":                    "", // if current mysql is also used for hive
		"default":                 "", // if fetching default hive databases
	}
	for i := 0; i < len(resp); i++ {
		AssertContainsAny(a, expectedDBs, resp[i][0])
	}
}

func CaseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf("select * from %s.%s limit 2;", caseDB, caseTrainTable)
	head, rows, err := connectAndRunSQL(cmd)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
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

func CaseTrainSQL(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT *
FROM %s.%s
TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`, caseDB, caseTrainTable)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM %s.%s
PREDICT %s.%s.class
USING sqlflow_models.my_dnn_model;`, caseDB, caseTestTable, caseDB, casePredictTable)
	_, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s.%s LIMIT 5;`, caseDB, casePredictTable)
	_, rows, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}

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

func CaseTrainCustomModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := `SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model_custom;`
	_, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := `SELECT *
FROM iris.predict LIMIT 5;`
	_, rows, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[4], int64(0))
	}
}

func CaseTrainTextClassification(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainTextClassificationCustomLSTM(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train_processed
TRAIN sqlflow_models.StackedBiLSTMClassifier
WITH model.n_classes = 17, model.stack_units = [16], train.epoch = 1, train.batch_size = 32
COLUMN EMBEDDING(SEQ_CATEGORY_ID(news_title,1600,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_bilstm_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainSQLWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainDeepWideModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN DNNLinearCombinedClassifier
WITH model.n_classes = 3, model.dnn_hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2
COLUMN sepal_length, sepal_width FOR linear_feature_columns
COLUMN petal_length, petal_width FOR dnn_feature_columns
LABEL class
INTO sqlflow_models.my_dnn_linear_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainCustomModel tests using customized models
func CaseTrainCustomModelWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], train.batch_size = 10, train.epoch=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseSparseFeature(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM text_cn.train
TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainElasticDL is a case for training models using ElasticDL
func CaseTrainElasticDL(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT sepal_length, sepal_width, petal_length, petal_width, class
FROM %s.%s
TRAIN ElasticDLDNNClassifier
WITH
			model.optimizer = "optimizer",
			model.loss = "loss",
			model.eval_metrics_fn = "eval_metrics_fn",
			model.num_classes = 3,
			model.dataset_fn = "dataset_fn",
			train.shuffle = 120,
			train.epoch = 2,
			train.grads_to_wait = 2,
			train.tensorboard_log_dir = "",
			train.checkpoint_steps = 0,
			train.checkpoint_dir = "",
			train.keep_checkpoint_max = 0,
			eval.steps = 0,
			eval.start_delay_secs = 100,
			eval.throttle_secs = 0,
			eval.checkpoint_filename_for_init = "",
			engine.master_resource_request = "cpu=400m,memory=1024Mi",
			engine.master_resource_limit = "cpu=1,memory=2048Mi",
			engine.worker_resource_request = "cpu=400m,memory=2048Mi",
			engine.worker_resource_limit = "cpu=1,memory=3072Mi",
			engine.num_workers = 2,
			engine.volume = "",
			engine.image_pull_policy = "Never",
			engine.restart_policy = "Never",
			engine.extra_pypi_index = "",
			engine.namespace = "default",
			engine.minibatch_size = 64,
			engine.master_pod_priority = "",
			engine.cluster_spec = "",
			engine.num_minibatches_per_task = 2,
			engine.docker_image_repository = "",
			engine.envs = "",
			engine.job_name = "test-odps",
			engine.image_base = "elasticdl:ci"
COLUMN
			sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO trained_elasticdl_keras_classifier;`, os.Getenv("MAXCOMPUTE_PROJECT"), "sqlflow_test_iris_train")
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPS is a case for training models using ALPS with out feature_map table
func CaseTrainALPS(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT deep_id, user_space_stat, user_behavior_stat, space_stat, l
FROM %s.sparse_column_test
LIMIT 100
TRAIN DNNClassifier
WITH model.n_classes = 2, model.hidden_units = [10, 20], train.batch_size = 10, engine.ps_num=0, engine.worker_num=0, engine.type=local
COLUMN SPARSE(deep_id,15033,COMMA,int),
       SPARSE(user_space_stat,310,COMMA,int),
       SPARSE(user_behavior_stat,511,COMMA,int),
       SPARSE(space_stat,418,COMMA,int),
       EMBEDDING(CATEGORY_ID(deep_id,15033,COMMA),512,mean),
       EMBEDDING(CATEGORY_ID(user_space_stat,310,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(user_behavior_stat,511,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(space_stat,418,COMMA),64,mean)
LABEL l
INTO model_table;`, caseDB)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPSRemoteModel is a case for training models using ALPS with remote model
func CaseTrainALPSRemoteModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT deep_id, user_space_stat, user_behavior_stat, space_stat, l
FROM %s.sparse_column_test
LIMIT 100
TRAIN models.estimator.dnn_classifier.DNNClassifier
WITH 
	model.n_classes = 2, model.hidden_units = [10, 20], train.batch_size = 10, engine.ps_num=0, engine.worker_num=0, engine.type=local,
	gitlab.project = "Alps/sqlflow-models",
	gitlab.source_root = python,
	gitlab.token = "%s"
COLUMN SPARSE(deep_id,15033,COMMA,int),
       SPARSE(user_space_stat,310,COMMA,int),
       SPARSE(user_behavior_stat,511,COMMA,int),
       SPARSE(space_stat,418,COMMA,int),
       EMBEDDING(CATEGORY_ID(deep_id,15033,COMMA),512,mean),
       EMBEDDING(CATEGORY_ID(user_space_stat,310,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(user_behavior_stat,511,COMMA),64,mean),
       EMBEDDING(CATEGORY_ID(space_stat,418,COMMA),64,mean)
LABEL l
INTO model_table;`, caseDB, os.Getenv("GITLAB_TOKEN"))
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPSFeatureMap is a case for training models using ALPS with feature_map table
func CaseTrainALPSFeatureMap(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT dense, deep, item, test_sparse_with_fm.label
FROM %s.test_sparse_with_fm
LIMIT 32
TRAIN alipay.SoftmaxClassifier
WITH train.max_steps = 32, eval.steps=32, train.batch_size=8, engine.ps_num=0, engine.worker_num=0, engine.type = local
COLUMN DENSE(dense, none, comma),
       DENSE(item, 1, comma, int)
LABEL "label" INTO model_table;`, caseDB)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseSQLByPassLeftJoin is a case for testing left join
func CaseSQLByPassLeftJoin(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT f1.user_id, f1.fea1, f2.fea2
FROM standard_join_test.user_fea1 AS f1 LEFT OUTER JOIN standard_join_test.user_fea2 AS f2
ON f1.user_id = f2.user_id
WHERE f1.user_id < 3;`

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
	// wait train finish
	ParseRow(stream)
}

// CaseTrainRegression is used to test regression models
func CaseTrainRegression(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT *
FROM housing.train
TRAIN LinearRegressor
WITH model.label_dimension=1
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_regression_model;`)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
PREDICT housing.predict.target
USING sqlflow_models.my_regression_model;`)
	_, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.predict LIMIT 5;`)
	_, rows, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[13], float64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 13 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 13)
	}
}

// CaseTrainXGBoostRegression is used to test xgboost regression models
func CaseTrainXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
SELECT *
FROM housing.train
TRAIN xgboost.gbtree
WITH
		objective="reg:squarederror",
		train.num_boost_round = 30
		COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
`)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CasePredictXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
PREDICT housing.xgb_predict.target
USING sqlflow_models.my_xgb_regression_model;`)
	_, _, err := connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.xgb_predict LIMIT 5;`)
	_, rows, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[13], float64(0))

		// avoiding nil features in predict result
		nilCount := 0
		for ; nilCount < 13 && row[nilCount] == nil; nilCount++ {
		}
		a.False(nilCount == 13)
	}
}
