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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/testdata"
)

var dbConnStr string

var caseDB = "iris"
var caseTrainTable = caseDB + ".train"
var caseTestTable = caseDB + ".test"
var casePredictTable = caseDB + ".predict"
var testDatasource = os.Getenv("SQLFLOW_TEST_DATASOURCE")

// caseInto is used by function CaseTrainSQL in this file. When
// testing with MaxCompute, the project is pre-created, we only need to
// specify the table name in that case.
var caseInto = "sqlflow_models.my_dnn_model"

const unitTestPort = 50051

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

func connectAndRunSQLShouldError(sql string) {
	conn, err := createRPCConn()
	if err != nil {
		log.Fatalf("connectAndRunSQLShouldError: %v", err)
	}
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()
	stream, err := cli.Run(ctx, sqlRequest(sql))
	if err != nil {
		log.Fatalf("connectAndRunSQLShouldError: %v", err)
	}
	_, err = stream.Recv()
	if err == nil {
		log.Fatalf("connectAndRunSQLShouldError: the statement should error")
	}
}

func connectAndRunSQL(sql string) ([]string, [][]*any.Any, []string, error) {
	conn, err := createRPCConn()
	if err != nil {
		return nil, nil, nil, err
	}
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)
	// PAI tests may take a long time until the cluster resource is ready, increase the RPC deadline here.
	ctx, cancel := context.WithTimeout(context.Background(), 36000*time.Second)
	defer cancel()
	stream, err := cli.Run(ctx, sqlRequest(sql))
	if err != nil {
		return nil, nil, nil, err
	}
	cols, rows, messages := ParseResponse(stream)
	return cols, rows, messages, nil
}

func sqlRequest(sql string) *pb.Request {
	se := &pb.Session{
		Token:            "user-unittest",
		DbConnStr:        dbConnStr,
		HdfsNamenodeAddr: os.Getenv("SQLFLOW_TEST_NAMENODE_ADDR"),
	}
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
		a.GreaterOrEqual(b.Value, float32(expected.(float64)))
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

func AssertIsSubStringAny(a *assert.Assertions, substring string, actual *any.Any) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.StringValue":
		b := wrappers.StringValue{}
		ptypes.UnmarshalAny(actual, &b)
		if !strings.Contains(b.Value, substring) {
			a.Failf("", "%s have no sub string: %s", b.Value, substring)
		}
	}
}

func ParseResponse(stream pb.SQLFlow_RunClient) ([]string, [][]*any.Any, []string) {
	var rows [][]*any.Any
	var columns []string
	var messages []string
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
		if iter.GetMessage() != nil {
			messages = append(messages, iter.GetMessage().Message)
		}
		counter++
	}
	return columns, rows, messages
}

func prepareTestData(dbStr string) error {
	testDB, e := database.OpenAndConnectDB(dbStr)
	if e != nil {
		return e
	}

	db := os.Getenv("SQLFLOW_TEST_DB")
	if db != "maxcompute" && db != "alisa" {
		_, e := testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
		if e != nil {
			return e
		}
	}

	var datasets []string
	switch db {
	case "mysql":
		datasets = []string{
			testdata.IrisSQL,
			testdata.ChurnSQL,
			testdata.StandardJoinTest,
			testdata.HousingSQL,
			testdata.FeatureDerivationCaseSQL,
			testdata.TextCNSQL}
	case "hive":
		datasets = []string{
			testdata.IrisHiveSQL,
			testdata.ChurnHiveSQL,
			testdata.FeatureDerivationCaseSQLHive,
			testdata.HousingSQL}
	case "maxcompute", "alisa":
		if os.Getenv("SQLFLOW_submitter") == "alps" {
			datasets = []string{
				testdata.ODPSFeatureMapSQL,
				testdata.ODPSSparseColumnSQL,
				fmt.Sprintf(testdata.IrisMaxComputeSQL, caseDB)}
		} else {
			datasets = []string{fmt.Sprintf(testdata.IrisMaxComputeSQL, caseDB)}
		}
	default:
		return fmt.Errorf("unrecognized SQLFLOW_TEST_DB %s", db)
	}

	for _, dataset := range datasets {
		if err := testdata.Popularize(testDB.DB, dataset); err != nil {
			return err
		}
	}
	return nil
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
		return grpc.Dial(fmt.Sprintf("localhost:%d", unitTestPort), grpc.WithTransportCredentials(creds))
	}
	return grpc.Dial(fmt.Sprintf("localhost:%d", unitTestPort), grpc.WithInsecure())
}

func TestEnd2EndMySQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	dbConnStr = "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("CaseShowDatabases", CaseShowDatabases)
	t.Run("CaseSelect", CaseSelect)
	t.Run("CaseEmptyDataset", CaseEmptyDataset)
	t.Run("CaseLabelColumnNotExist", CaseLabelColumnNotExist)
	t.Run("CaseTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainAndEvaluate", CaseTrainAndEvaluate)
	t.Run("CaseTrainPredictCategoricalFeature", CaseTrainPredictCategoricalFeature)
	t.Run("CaseTrainRegex", CaseTrainRegex)
	t.Run("CaseTypoInColumnClause", CaseTypoInColumnClause)
	t.Run("CaseTrainWithCommaSeparatedLabel", CaseTrainWithCommaSeparatedLabel)

	t.Run("CaseTrainBoostedTreesEstimatorAndExplain", CaseTrainBoostedTreesEstimatorAndExplain)
	t.Run("CaseTrainSQLWithMetrics", CaseTrainSQLWithMetrics)
	t.Run("TestTextClassification", CaseTrainTextClassification)
	t.Run("CaseTrainTextClassificationCustomLSTM", CaseTrainTextClassificationCustomLSTM)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainCustomModelFunctional", CaseTrainCustomModelFunctional)
	t.Run("CaseTrainOptimizer", CaseTrainOptimizer)
	t.Run("CaseTrainSQLWithHyperParams", CaseTrainSQLWithHyperParams)
	t.Run("CaseTrainCustomModelWithHyperParams", CaseTrainCustomModelWithHyperParams)
	t.Run("CaseSparseFeature", CaseSparseFeature)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", CaseTrainRegression)
	t.Run("CaseTrainXGBoostRegression", CaseTrainXGBoostRegression)
	t.Run("CaseTrainXGBoostMultiClass", CaseTrainXGBoostMultiClass)

	t.Run("CasePredictXGBoostRegression", CasePredictXGBoostRegression)
	t.Run("CaseTrainAndExplainXGBoostModel", CaseTrainAndExplainXGBoostModel)

	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
	t.Run("CaseTrainDeepWideModelOptimizer", CaseTrainDeepWideModelOptimizer)
	t.Run("CaseTrainAdaNetAndExplain", CaseTrainAdaNetAndExplain)

	// Cases using feature derivation
	t.Run("CaseTrainTextClassificationIR", CaseTrainTextClassificationIR)
	t.Run("CaseTrainTextClassificationFeatureDerivation", CaseTrainTextClassificationFeatureDerivation)
	t.Run("CaseXgboostFeatureDerivation", CaseXgboostFeatureDerivation)
	t.Run("CaseXgboostEvalMetric", CaseXgboostEvalMetric)
	t.Run("CaseXgboostExternalMemory", CaseXgboostExternalMemory)
	t.Run("CaseTrainFeatureDerivation", CaseTrainFeatureDerivation)

	t.Run("CaseShowTrain", CaseShowTrain)
}

func CaseEmptyDataset(t *testing.T) {
	trainSQL := `SELECT * FROM iris.train LIMIT 0 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL class 
INTO sqlflow_models.my_xgb_regression_model;`
	connectAndRunSQLShouldError(trainSQL)
}

func CaseLabelColumnNotExist(t *testing.T) {
	trainSQL := `SELECT * FROM iris.train WHERE class=2 TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror"
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`
	connectAndRunSQLShouldError(trainSQL)
}

func CaseXgboostFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM housing.train
TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror",
	 train.num_boost_round=30
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM housing.test
TO PREDICT housing.predict.target
USING sqlflow_models.my_xgb_regression_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseXgboostEvalMetric(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train WHERE class in (0, 1) TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", eval_metric=auc
LABEL class
INTO sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM iris.test TO PREDICT iris.predict.class
USING sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseXgboostExternalMemory(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train WHERE class in (0, 1) TO TRAIN xgboost.gbtree
WITH objective="binary:logistic", eval_metric=auc, train.disk_cache=True
LABEL class
INTO sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM iris.test TO PREDICT iris.predict.class
USING sqlflow_models.my_xgb_binary_classification_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}
}

func CaseTrainTextClassificationIR(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(SPARSE(news_title,16000,COMMA), 16000),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
}

func CaseTrainTextClassificationFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
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
	dbConnStr = "hive://root:root@127.0.0.1:10000/iris?auth=NOSASL"
	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainSQLWithMetrics", CaseTrainSQLWithMetrics)
	t.Run("CaseTrainRegression", CaseTrainRegression)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainAdaNetAndExplain", CaseTrainAdaNetAndExplain)
	t.Run("CaseTrainOptimizer", CaseTrainOptimizer)
	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
	t.Run("CaseTrainDeepWideModelOptimizer", CaseTrainDeepWideModelOptimizer)
	t.Run("CaseTrainXGBoostRegression", CaseTrainXGBoostRegression)
	t.Run("CasePredictXGBoostRegression", CasePredictXGBoostRegression)
	t.Run("CaseTrainFeatureDerivation", CaseTrainFeatureDerivation)
	t.Run("CaseShowTrain", CaseShowTrain)
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
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	caseTrainTable = "sqlflow_test_iris_train"
	caseTestTable = "sqlflow_test_iris_test"
	casePredictTable = "sqlflow_test_iris_predict"
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

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
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT when testing ALPS cases (SQLFLOW_submitter=alps)!!")
	}
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	t.Run("CaseTrainALPS", CaseTrainALPS)
	t.Run("CaseTrainALPSFeatureMap", CaseTrainALPSFeatureMap)
	t.Run("CaseTrainALPSRemoteModel", CaseTrainALPSRemoteModel)
}

// TODO(typhoonzero): add back below tests when done ElasticDL refactoring.
// func TestEnd2EndMaxComputeElasticDL(t *testing.T) {
// 	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
// 	modelDir, _ := ioutil.TempDir("/tmp", "sqlflow_ssl_")
// 	defer os.RemoveAll(modelDir)
// 	tmpDir, caCrt, caKey, err := generateTempCA()
// 	defer os.RemoveAll(tmpDir)
// 	if err != nil {
// 		t.Fatalf("failed to generate CA pair %v", err)
// 	}
// 	submitter := os.Getenv("SQLFLOW_submitter")
// 	if submitter != "elasticdl" {
// 		t.Skip("Skip, this test is for maxcompute + ElasticDL")
// 	}

// 	if testDBDriver != "maxcompute" {
// 		t.Skip("Skip maxcompute tests")
// 	}
// 	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
// 	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
// 	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
// 	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)

// 	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
// 	if caseDB == "" {
// 		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT when testing ElasticDL cases (SQLFLOW_submitter=elasticdl)!!")
// 	}
// 	err = prepareTestData(dbConnStr)
// 	if err != nil {
// 		t.Fatalf("prepare test dataset failed: %v", err)
// 	}

// 	go start(modelDir, caCrt, caKey, unitTestPort, false)
// 	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

// 	t.Run("CaseTrainElasticDL", CaseTrainElasticDL)
// }

// // CaseTrainElasticDL is a case for training models using ElasticDL
// func CaseTrainElasticDL(t *testing.T) {
// 	a := assert.New(t)
// 	trainSQL := fmt.Sprintf(`SELECT sepal_length, sepal_width, petal_length, petal_width, class
// FROM %s.%s
// TO TRAIN ElasticDLDNNClassifier
// WITH
// 			model.optimizer = "optimizer",
// 			model.loss = "loss",
// 			model.eval_metrics_fn = "eval_metrics_fn",
// 			model.num_classes = 3,
// 			model.dataset_fn = "dataset_fn",
// 			train.shuffle = 120,
// 			train.epoch = 2,
// 			train.grads_to_wait = 2,
// 			train.tensorboard_log_dir = "",
// 			train.checkpoint_steps = 0,
// 			train.checkpoint_dir = "",
// 			train.keep_checkpoint_max = 0,
// 			eval.steps = 0,
// 			eval.start_delay_secs = 100,
// 			eval.throttle_secs = 0,
// 			eval.checkpoint_filename_for_init = "",
// 			engine.master_resource_request = "cpu=400m,memory=1024Mi",
// 			engine.master_resource_limit = "cpu=1,memory=2048Mi",
// 			engine.worker_resource_request = "cpu=400m,memory=2048Mi",
// 			engine.worker_resource_limit = "cpu=1,memory=3072Mi",
// 			engine.num_workers = 2,
// 			engine.volume = "",
// 			engine.image_pull_policy = "Never",
// 			engine.restart_policy = "Never",
// 			engine.extra_pypi_index = "",
// 			engine.namespace = "default",
// 			engine.minibatch_size = 64,
// 			engine.master_pod_priority = "",
// 			engine.cluster_spec = "",
// 			engine.num_minibatches_per_task = 2,
// 			engine.docker_image_repository = "",
// 			engine.envs = "",
// 			engine.job_name = "test-odps",
// 			engine.image_base = "elasticdl:ci"
// COLUMN
// 			sepal_length, sepal_width, petal_length, petal_width
// LABEL class
// INTO trained_elasticdl_keras_classifier;`, os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT"), "sqlflow_test_iris_train")
// 	_, _, _, err := connectAndRunSQL(trainSQL)
// 	if err != nil {
// 		a.Fail("run trainSQL error: %v", err)
// 	}
// }

func CaseShowDatabases(t *testing.T) {
	a := assert.New(t)
	cmd := "show databases;"
	head, resp, _, err := connectAndRunSQL(cmd)
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
		"sanity_check":            "",
		"iris_e2e":                "", // created by Python e2e test
		"hive":                    "", // if current mysql is also used for hive
		"default":                 "", // if fetching default hive databases
		"sqlflow":                 "", // to save model zoo trained models
	}
	for i := 0; i < len(resp); i++ {
		AssertContainsAny(a, expectedDBs, resp[i][0])
	}
}

func CaseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf("select * from %s limit 2;", caseTrainTable)
	head, rows, _, err := connectAndRunSQL(cmd)
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

func CaseTrainPredictCategoricalFeature(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT f9, target FROM housing.train
TO TRAIN DNNRegressor WITH
		model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 1000), 2, "sum")
LABEL target
INTO housing.dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `SELECT f9, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	trainSQL = `SELECT f9, f10, target FROM housing.train
TO TRAIN DNNLinearCombinedRegressor WITH
		model.dnn_hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(f9, 25), 2, "sum") for dnn_feature_columns
COLUMN INDICATOR(CATEGORY_ID(f10, 712)) for linear_feature_columns
LABEL target
INTO housing.dnnlinear_model;`
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL = `SELECT f9, f10, target FROM housing.test
TO PREDICT housing.predict.class USING housing.dnnlinear_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

func CaseTrainAndEvaluate(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class<>2
TO TRAIN DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM %s WHERE class <>2 LIMIT 30"
LABEL class
INTO %s;`, caseTrainTable, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class<>2
TO EVALUATE %s
WITH validation.metrics = "Accuracy,AUC"
LABEL class
INTO %s.evaluation_result;`, caseTestTable, caseInto, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CaseTrainSQL(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select = "SELECT * FROM %s LIMIT 30"
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM %s
TO PREDICT %s.class
USING %s;`, caseTestTable, casePredictTable, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s LIMIT 5;`, casePredictTable)
	_, rows, _, err := connectAndRunSQL(showPred)
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

func CaseTrainRegex(t *testing.T) {
	a := assert.New(t)
	trainSQL := `
SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM housing.test"
COLUMN INDICATOR(CATEGORY_ID("f10|f9|f4", 1000))
LABEL target
INTO housing.dnn_model;
`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `
	SELECT * FROM housing.test
	TO PREDICT housing.predict.class
	USING housing.dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
	trainSQL = `
SELECT * FROM housing.train
TO TRAIN DNNRegressora WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("a.*", 1000))
LABEL target
INTO housing.dnn_model;
` // don't match any column
	connectAndRunSQLShouldError(trainSQL)

	trainSQL = `
SELECT * FROM housing.train
TO TRAIN DNNRegressor WITH
    model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM %s "
COLUMN INDICATOR(CATEGORY_ID("[*", 1000))
LABEL target
INTO housing.dnn_model;
` // invalid regex
	connectAndRunSQLShouldError(trainSQL)

}

func CaseTypoInColumnClause(t *testing.T) {
	trainSQL := fmt.Sprintf(`
	SELECT * FROM %s
	TO TRAIN DNNClassifier WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select = "SELECT * FROM %s LIMIT 30"
	COLUMN typo, sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseTrainTable, caseInto)
	connectAndRunSQLShouldError(trainSQL)
}

func CaseTrainBoostedTreesEstimatorAndExplain(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
	SELECT * FROM iris.train WHERE class!=2
	TO TRAIN BoostedTreesClassifier
	WITH
		model.n_batches_per_layer=1,
		model.center_bias=True,
		train.batch_size=100,
		train.epoch=10,
		validation.select="SELECT * FROM iris.test where class!=2"
	LABEL class
	INTO %s;
	`, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM iris.test WHERE class!=2
	TO EXPLAIN %s
	INTO iris.explain_result;`, caseInto)
	_, _, _, err = connectAndRunSQL(explainSQL)
	a.NoError(err)

	getExplainResult := `SELECT * FROM iris.explain_result;`
	_, rows, _, err := connectAndRunSQL(getExplainResult)
	a.NoError(err)
	for _, row := range rows {
		AssertGreaterEqualAny(a, row[1], float32(0))
	}
}

func CaseTrainLinearClassifier(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM iris.train WHERE class !=2
TO TRAIN LinearClassifier LABEL class INTO %s;`, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CaseTrainSQLWithMetrics(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train WHERE class!=2
TO TRAIN DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 10],
	train.batch_size = 4,
	validation.select = "SELECT * FROM iris.test WHERE class!=2",
	validation.metrics = "Accuracy,AUC"
LABEL class
INTO sqlflow_models.mytest_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	// TODO(shendiaomo): sqlflow_models.DNNClassifier.eval_metrics_fn only works when batch_size is 1
	kerasTrainSQL := `SELECT * FROM iris.train WHERE class!=2
TO TRAIN sqlflow_models.DNNClassifier
WITH
	model.n_classes = 2,
	model.hidden_units = [10, 10],
	train.batch_size = 1,
	validation.select = "SELECT * FROM iris.test WHERE class!=2",
	validation.metrics = "Accuracy,AUC,Precision,Recall"
LABEL class
INTO sqlflow_models.mytest_model;`
	_, _, _, err = connectAndRunSQL(kerasTrainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	regressionTrainSQL := `SELECT * FROM housing.train
TO TRAIN DNNRegressor
WITH
	model.hidden_units = [10, 10],
	train.batch_size = 4,
	validation.select = "SELECT * FROM housing.test",
	validation.metrics = "MeanAbsoluteError,MeanAbsolutePercentageError,MeanSquaredError"
LABEL target
INTO sqlflow_models.myreg_model;`
	_, _, _, err = connectAndRunSQL(regressionTrainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CaseTrainFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)

	predSQL := `SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	a.NoError(err)

	// TODO(typhoonzero): also support string column type for training and prediction (column c6)
	// NOTE(typhoonzero): this test also tests saving to the same model name when saving to model zoo table (sqlflow.trained_models)
	trainVaryColumnTypes := `SELECT c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
COLUMN EMBEDDING(c3, 32, sum), EMBEDDING(SPARSE(c5, 64, COMMA), 32, sum)
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err = connectAndRunSQL(trainVaryColumnTypes)
	a.NoError(err)

	trainVaryColumnTypes = `SELECT c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
COLUMN INDICATOR(c3), EMBEDDING(SPARSE(c5, 64, COMMA), 32, sum)
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err = connectAndRunSQL(trainVaryColumnTypes)
	a.NoError(err)

}

func CaseTrainOptimizer(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], model.optimizer=RMSprop
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)

	predSQL := `SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`
	_, _, _, err = connectAndRunSQL(predSQL)
	a.NoError(err)

	trainKerasSQL := `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 model.optimizer=RMSprop, optimizer.learning_rate=0.1,
	 model.loss=SparseCategoricalCrossentropy
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err = connectAndRunSQL(trainKerasSQL)
	a.NoError(err)
}

func CaseTrainCustomModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], validation.select="select * from %s", validation.steps=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;`, caseTrainTable, caseTestTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.class
USING %s;`, caseTestTable, casePredictTable, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT * FROM %s LIMIT 5;`, casePredictTable)
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: predict result maybe random, only check predicted
		// class >=0, need to change to more flexible checks than
		// checking expectedPredClasses := []int64{2, 1, 0, 2, 0}
		AssertGreaterEqualAny(a, row[4], int64(0))
	}

	trainSQL = fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH model.n_classes = 3, validation.metrics="CategoricalAccuracy"
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;`, caseTrainTable, caseInto)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainCustomModelFunctional(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH model.n_classes = 3, validation.metrics="CategoricalAccuracy"
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;`, caseTrainTable, caseInto)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainWithCommaSeparatedLabel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT sepal_length, sepal_width, petal_length, concat(petal_width,',',class) as class FROM iris.train 
	TO TRAIN sqlflow_models.LSTMBasedTimeSeriesModel WITH
	  model.n_in=3,
	  model.stack_units = [10, 10],
	  model.n_out=2,
	  validation.metrics= "MeanAbsoluteError,MeanSquaredError"
	LABEL class
	INTO sqlflow_models.my_dnn_regts_model_2;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := `SELECT sepal_length, sepal_width, petal_length, concat(petal_width,',',class) as class FROM iris.test 
	TO PREDICT iris.predict_ts_2.class USING sqlflow_models.my_dnn_regts_model_2;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	showPred := `SELECT * FROM iris.predict_ts_2 LIMIT 5;`
	_, rows, _, err := connectAndRunSQL(showPred)
	if err != nil {
		a.Fail("Run showPred error: %v", err)
	}

	for _, row := range rows {
		// NOTE: Ensure that the predict result contains comma
		AssertIsSubStringAny(a, ",", row[3])
	}
}

func CaseTrainTextClassification(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN DNNClassifier
WITH model.n_classes = 17, model.hidden_units = [10, 20]
COLUMN EMBEDDING(CATEGORY_ID(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainTextClassificationCustomLSTM(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train_processed
TO TRAIN sqlflow_models.StackedBiLSTMClassifier
WITH model.n_classes = 17, model.stack_units = [16], train.epoch = 1, train.batch_size = 32
COLUMN EMBEDDING(SEQ_CATEGORY_ID(news_title,1600,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_bilstm_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainSQLWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 train.batch_size = 10, train.epoch = 6,
	 train.max_steps = 200,
	 train.save_checkpoints_steps=10,
	 train.log_every_n_iter=20,
	 validation.start_delay_secs=10, validation.throttle_secs=10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainDeepWideModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNLinearCombinedClassifier
WITH model.n_classes = 3, model.dnn_hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2
COLUMN sepal_length, sepal_width FOR linear_feature_columns
COLUMN petal_length, petal_width FOR dnn_feature_columns
LABEL class
INTO sqlflow_models.my_dnn_linear_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainAdaNetAndExplain(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train
TO TRAIN sqlflow_models.AutoClassifier WITH model.n_classes = 3 LABEL class INTO sqlflow_models.my_adanet_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
	explainSQL := `SELECT * FROM iris.test LIMIT 10 TO EXPLAIN sqlflow_models.my_adanet_model;`
	_, _, _, err = connectAndRunSQL(explainSQL)
	a.NoError(err)
}

func CaseTrainDeepWideModelOptimizer(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNLinearCombinedClassifier
WITH model.n_classes = 3, model.dnn_hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2,
model.dnn_optimizer=RMSprop, dnn_optimizer.learning_rate=0.01
COLUMN sepal_length, sepal_width FOR linear_feature_columns
COLUMN petal_length, petal_width FOR dnn_feature_columns
LABEL class
INTO sqlflow_models.my_dnn_linear_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainCustomModel tests using customized models
func CaseTrainCustomModelWithHyperParams(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], train.batch_size = 10, train.epoch=2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseSparseFeature(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

// CaseTrainALPS is a case for training models using ALPS with out feature_map table
func CaseTrainALPS(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
	SELECT deep_id, user_space_stat, user_behavior_stat, space_stat, l
	FROM %s.sparse_column_test
	LIMIT 100
	TO TRAIN DNNClassifier
	WITH
	    model.n_classes = 2,
	    model.hidden_units = [10, 20],
	    train.batch_size = 10,
	    engine.ps_num = 0,
	    engine.worker_num = 0,
	    engine.type = local,
	    validation.table = "%s.sparse_column_test"
	COLUMN
	    SPARSE(deep_id,15033,COMMA,int),
	    SPARSE(user_space_stat,310,COMMA,int),
	    SPARSE(user_behavior_stat,511,COMMA,int),
	    SPARSE(space_stat,418,COMMA,int),
	    EMBEDDING(CATEGORY_ID(deep_id,15033,COMMA),512,mean),
	    EMBEDDING(CATEGORY_ID(user_space_stat,310,COMMA),64,mean),
	    EMBEDDING(CATEGORY_ID(user_behavior_stat,511,COMMA),64,mean),
	    EMBEDDING(CATEGORY_ID(space_stat,418,COMMA),64,mean)
	LABEL l
	INTO model_table;
	`, caseDB, caseDB)
	_, _, _, err := connectAndRunSQL(trainSQL)
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
TO TRAIN models.estimator.dnn_classifier.DNNClassifier
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
	_, _, _, err := connectAndRunSQL(trainSQL)
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
TO TRAIN alipay.SoftmaxClassifier
WITH train.max_steps = 32, eval.steps=32, train.batch_size=8, engine.ps_num=0, engine.worker_num=0, engine.type = local
COLUMN DENSE(dense, none, comma),
       DENSE(item, 1, comma, int)
LABEL "label" INTO model_table;`, caseDB)
	_, _, _, err := connectAndRunSQL(trainSQL)
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
	ParseResponse(stream)
}

// CaseTrainRegression is used to test regression models
func CaseTrainRegression(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT *
FROM housing.train
TO TRAIN LinearRegressor
WITH model.label_dimension=1
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_regression_model;`)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
TO PREDICT housing.predict.result
USING sqlflow_models.my_regression_model;`)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.predict LIMIT 5;`)
	_, rows, _, err := connectAndRunSQL(showPred)
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
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	scale_pos_weight=2,
	train.num_boost_round = 30,
	validation.select="SELECT * FROM housing.train LIMIT 20"
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
`)
	_, _, messages, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	isConvergence := false
	reLog := regexp.MustCompile(`.*29.*train-rmse:(.+)?validate-rmse\:(.+)?`)
	for _, msg := range messages {
		sub := reLog.FindStringSubmatch(msg)
		if len(sub) == 3 {
			trainRmse, e := strconv.ParseFloat(strings.TrimSpace(sub[1]), 32)
			a.NoError(e)
			valRmse, e := strconv.ParseFloat(strings.TrimSpace(sub[2]), 32)
			a.NoError(e)
			a.Greater(trainRmse, 0.0)            // no overfitting
			a.LessOrEqual(trainRmse, 0.5)        // less the baseline
			a.GreaterOrEqual(valRmse, trainRmse) // verify the validation
			isConvergence = true
		}
	}
	a.Truef(isConvergence, strings.Join(messages, "\n"))

	evalSQL := fmt.Sprintf(`
SELECT * FROM housing.train
TO EVALUATE sqlflow_models.my_xgb_regression_model
WITH validation.metrics="mean_absolute_error,mean_squared_error"
LABEL target
INTO sqlflow_models.my_xgb_regression_model_eval_result;
`)
	_, _, messages, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("run evalSQL error: %v", err)
	}
}

// CaseTrainXGBoostMultiClass is used to test xgboost regression models
func CaseTrainXGBoostMultiClass(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
SELECT *
FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softprob",
	num_class=3,
	validation.select="SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.my_xgb_multi_class_model;
`)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`
SELECT * FROM iris.test
TO EVALUATE sqlflow_models.my_xgb_multi_class_model
WITH validation.metrics="accuracy_score"
LABEL class
INTO sqlflow_models.my_xgb_regression_model_eval_result;
`)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("run evalSQL error: %v", err)
	}
}

// CaseTrainAndExplainXGBoostModel is used to test training a xgboost model,
// then explain it
func CaseTrainAndExplainXGBoostModel(t *testing.T) {
	a := assert.New(t)
	trainStmt := `
SELECT *
FROM housing.train
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	train.num_boost_round = 30,
	train.batch_size=20
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
	`
	explainStmt := `
SELECT *
FROM housing.train
TO EXPLAIN sqlflow_models.my_xgb_regression_model
WITH
    summary.plot_type="bar",
    summary.alpha=1,
    summary.sort=True
USING TreeExplainer;
	`
	conn, err := createRPCConn()
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, sqlRequest(trainStmt))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	ParseResponse(stream)
	stream, err = cli.Run(ctx, sqlRequest(explainStmt))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	ParseResponse(stream)
}

func CasePredictXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
TO PREDICT housing.xgb_predict.target
USING sqlflow_models.my_xgb_regression_model;`)
	_, _, _, err := connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM housing.xgb_predict LIMIT 5;`)
	_, rows, _, err := connectAndRunSQL(showPred)
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

func CasePAIMaxComputeTrainPredictCategoricalFeature(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := `SELECT cast(sepal_length as int) sepal_length, class
FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNClassifier WITH
		model.hidden_units = [10, 20], model.n_classes=3
COLUMN EMBEDDING(CATEGORY_ID(sepal_length, 1000), 2, "sum")
LABEL class
INTO e2etest_predict_categorical_feature;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := `SELECT cast(sepal_length as int) sepal_length, class FROM alifin_jtest_dev.sqlflow_test_iris_test
TO PREDICT alifin_jtest_dev.pred_catcol.class USING e2etest_predict_categorical_feature;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	trainSQL = `SELECT cast(sepal_length as int) sepal_length, cast(sepal_width as int) sepal_width, class
FROM alifin_jtest_dev.sqlflow_test_iris_train
TO TRAIN DNNLinearCombinedClassifier WITH
		model.dnn_hidden_units = [10, 20], model.n_classes=3
COLUMN EMBEDDING(CATEGORY_ID(sepal_length, 20), 2, "sum") for dnn_feature_columns
COLUMN EMBEDDING(CATEGORY_ID(sepal_width, 20), 2, "sum") for linear_feature_columns
LABEL class
INTO e2etest_predict_categorical_feature2;`
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL = `SELECT cast(sepal_length as int) sepal_length, cast(sepal_width as int) sepal_width, class FROM alifin_jtest_dev.sqlflow_test_iris_test
TO PREDICT alifin_jtest_dev.pred_catcol2.class USING e2etest_predict_categorical_feature2;`
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	train.num_workers=2,
	train.num_ps=2,
	train.save_checkpoints_steps=20,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1
LABEL class
INTO e2etest_dnn_model_distributed;`, caseTrainTable)
	connectAndRunSQLShouldError(trainSQL)

	trainSQL = fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	train.num_workers=2,
	train.num_ps=2,
	train.save_checkpoints_steps=20,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1,
	validation.select="select * from %s"
LABEL class
INTO e2etest_dnn_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CasePAIMaxComputeTrainDistributedKeras(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.dnnclassifier_functional_model
WITH
	model.n_classes=3,
	train.num_workers=2,
	train.num_ps=2,
	train.epoch=10,
	train.batch_size=4,
	train.verbose=1,
	validation.select="select * from %s",
	validation.metrics="CategoricalAccuracy"
LABEL class
INTO e2etest_keras_dnn_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CasePAIMaxComputeTrainXGBDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 30,
		train.num_workers = 2,
		eta = 0.4,
		num_class = 3,
		train.batch_size=10,
		validation.select="select * from %s"
	LABEL class
	INTO e2etest_xgb_classi_model;`, caseTrainTable, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainTFBTDistributed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s WHERE class < 2
TO TRAIN BoostedTreesClassifier
WITH
	model.center_bias=True,
	model.n_batches_per_layer=70,
	train.num_workers=2,
	train.num_ps=1,
	train.epoch=10,
	validation.select="select * from %s"
LABEL class
INTO e2etest_tfbt_model_distributed;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)
}

func CaseTrainPAIKMeans(t *testing.T) {
	a := assert.New(t)
	err := dropPAIModel(dbConnStr, caseInto)
	a.NoError(err)

	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN kmeans 
	WITH
		center_count=3,
		idx_table_name=%s,
		excluded_columns=class
	INTO %s;
	`, caseTrainTable, caseTrainTable+"_test_output_idx", caseInto)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
	TO PREDICT %s.cluster_index
	USING %s;
	`, caseTestTable, casePredictTable, caseInto)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func dropPAIModel(dataSource, modelName string) error {
	code := fmt.Sprintf(`import subprocess
import sqlflow_submitter.db
driver, dsn = "%s".split("://")
assert driver == "maxcompute"
user, passwd, address, database = sqlflow_submitter.db.parseMaxComputeDSN(dsn)
cmd = "drop offlinemodel if exists %s"
subprocess.run(["odpscmd", "-u", user,
                           "-p", passwd,
                           "--project", database,
                           "--endpoint", address,
                           "-e", cmd],
               check=True)	
	`, dataSource, modelName)
	cmd := exec.Command("python", "-u")
	cmd.Stdin = bytes.NewBufferString(code)
	if e := cmd.Run(); e != nil {
		return e
	}
	return nil
}

func CaseTrainPAIRandomForests(t *testing.T) {
	a := assert.New(t)
	err := dropPAIModel(dbConnStr, "my_rf_model")
	a.NoError(err)

	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN randomforests
WITH tree_num = 3
LABEL class
INTO my_rf_model;`, caseTrainTable)
	_, _, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.class
USING my_rf_model;`, caseTestTable, casePredictTable)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
TO EXPLAIN my_rf_model
WITH label_column = class
USING TreeExplainer
INTO %s.rf_model_explain;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(explainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeDNNTrainPredictExplain(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
LABEL class
INTO e2etest_pai_dnn;`, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`SELECT * FROM %s
TO EVALUATE e2etest_pai_dnn
WITH validation.metrics="Accuracy,Recall"
LABEL class
INTO %s.e2etest_pai_dnn_evaluate_result;`, caseTrainTable, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.pai_dnn_predict.class
USING e2etest_pai_dnn;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s.pai_dnn_predict LIMIT 5;`, caseDB)
	_, rows, _, err := connectAndRunSQL(showPred)
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

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
TO EXPLAIN e2etest_pai_dnn
WITH label_col=class
USING TreeExplainer
INTO %s.pai_dnn_explain_result;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(explainSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainDenseCol(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	// Test train and predict using concated columns sepal_length, sepal_width, petal_length, petal_width
	trainSQL := fmt.Sprintf(`SELECT class, CONCAT(sepal_length, ",", sepal_width, ",", petal_length, ",", petal_width) AS f1
FROM %s
TO TRAIN DNNClassifier
WITH model.hidden_units=[64,32], model.n_classes=3, train.batch_size=32
COLUMN NUMERIC(f1, 4)
LABEL class
INTO e2etest_dense_input;`, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainXGBoost(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 30,
		eta = 0.4,
		num_class = 3,
		train.batch_size=10,
		validation.select="select * from %s"
	LABEL class
	INTO e2etest_xgb_classi_model;`, caseTrainTable, caseTrainTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.pai_xgb_predict.class
USING e2etest_xgb_classi_model;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	evalSQL := fmt.Sprintf(`SELECT * FROM %s
TO EVALUATE e2etest_xgb_classi_model
WITH validation.metrics="accuracy_score"
LABEL class
INTO %s.e2etest_xgb_evaluate_result;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(evalSQL)
	if err != nil {
		a.Fail("Run evalSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
TO EXPLAIN e2etest_xgb_classi_model
WITH label_col=class
USING TreeExplainer
INTO %s.e2etest_xgb_explain_result;`, caseTrainTable, caseDB)
	_, _, _, err = connectAndRunSQL(explainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

func CasePAIMaxComputeTrainCustomModel(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], validation.select="select * from %s", validation.steps=2
LABEL class
INTO e2etest_keras_dnn;`, caseTrainTable, caseTestTable)
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
TO PREDICT %s.keras_predict.class
USING e2etest_keras_dnn;`, caseTestTable, caseDB)
	_, _, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("run predSQL error: %v", err)
	}
}

func CaseTrainXGBoostOnAlisa(t *testing.T) {
	a := assert.New(t)
	model := "my_xgb_class_model"
	trainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO TRAIN xgboost.gbtree
	WITH
		objective="multi:softprob",
		train.num_boost_round = 30,
		eta = 0.4,
		num_class = 3
	LABEL class
	INTO %s;`, caseTrainTable, model)
	if _, _, _, err := connectAndRunSQL(trainSQL); err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT * FROM %s
	TO PREDICT %s.class
	USING %s;`, caseTestTable, casePredictTable, model)
	if _, _, _, err := connectAndRunSQL(predSQL); err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM %s
	TO EXPLAIN %s
	WITH label_col=class
	USING TreeExplainer
	INTO my_xgb_explain_result;`, caseTrainTable, model)
	if _, _, _, err := connectAndRunSQL(explainSQL); err != nil {
		a.Fail("Run predSQL error: %v", err)
	}
}

// TestEnd2EndAlisa test cases that run on Alisa, Need to set the
// below environment variables to run them:
// SQLFLOW_submitter=alisa
// SQLFLOW_TEST_DATASOURCE="xxx"
// SQLFLOW_OSS_CHECKPOINT_DIR="xxx"
// SQLFLOW_OSS_ALISA_ENDPOINT="xxx"
// SQLFLOW_OSS_AK="xxx"
// SQLFLOW_OSS_SK="xxx"
// SQLFLOW_OSS_ALISA_BUCKET="xxx"
// SQLFLOW_OSS_MODEL_ENDPOINT="xxx"
func TestEnd2EndAlisa(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	if testDBDriver != "alisa" {
		t.Skip("Skipping non alisa tests")
	}
	if os.Getenv("SQLFLOW_submitter") != "alisa" {
		t.Skip("Skip non Alisa tests")
	}
	dbConnStr = os.Getenv("SQLFLOW_TEST_DATASOURCE")
	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}
	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	}
	caseTrainTable = caseDB + ".sqlflow_test_iris_train"
	caseTestTable = caseDB + ".sqlflow_test_iris_test"
	casePredictTable = caseDB + ".sqlflow_test_iris_predict"
	// write model to current MaxCompute project
	caseInto = "sqlflow_test_kmeans_model"

	go start("", caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	// TODO(Yancey1989): reuse CaseTrainXGBoostOnPAI if support explain XGBoost model
	t.Run("CaseTrainXGBoostOnAlisa", CaseTrainXGBoostOnAlisa)
	t.Run("CaseTrainPAIKMeans", CaseTrainPAIKMeans)
}

// TestEnd2EndMaxComputePAI test cases that runs on PAI. Need to set below
// environment variables to run the test:
// SQLFLOW_submitter=pai
// SQLFLOW_TEST_DB=maxcompute
// SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_AK="xxx"
// SQLFLOW_TEST_DB_MAXCOMPUTE_SK="xxx"
// SQLFLOW_OSS_CHECKPOINT_DIR="xxx"
// SQLFLOW_OSS_ENDPOINT="xxx"
// SQLFLOW_OSS_AK="xxx"
// SQLFLOW_OSS_SK="xxx"
func TestEnd2EndMaxComputePAI(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	if testDBDriver != "maxcompute" {
		t.Skip("Skipping non maxcompute tests")
	}
	if os.Getenv("SQLFLOW_submitter") != "pai" {
		t.Skip("Skip non PAI tests")
	}
	AK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_AK")
	SK := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_SK")
	endpoint := os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_ENDPOINT")
	dbConnStr = fmt.Sprintf("maxcompute://%s:%s@%s", AK, SK, endpoint)
	modelDir := ""

	tmpDir, caCrt, caKey, err := generateTempCA()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatalf("failed to generate CA pair %v", err)
	}

	caseDB = os.Getenv("SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	if caseDB == "" {
		t.Fatalf("Must set env SQLFLOW_TEST_DB_MAXCOMPUTE_PROJECT")
	}
	caseTrainTable = caseDB + ".sqlflow_test_iris_train"
	caseTestTable = caseDB + ".sqlflow_test_iris_test"
	casePredictTable = caseDB + ".sqlflow_test_iris_predict"
	// write model to current MaxCompute project
	caseInto = "my_dnn_model"

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

	t.Run("group", func(t *testing.T) {
		t.Run("CasePAIMaxComputeDNNTrainPredictExplain", CasePAIMaxComputeDNNTrainPredictExplain)
		t.Run("CasePAIMaxComputeTrainDenseCol", CasePAIMaxComputeTrainDenseCol)
		t.Run("CasePAIMaxComputeTrainXGBoost", CasePAIMaxComputeTrainXGBoost)
		t.Run("CasePAIMaxComputeTrainCustomModel", CasePAIMaxComputeTrainCustomModel)
		t.Run("CasePAIMaxComputeTrainDistributed", CasePAIMaxComputeTrainDistributed)
		t.Run("CasePAIMaxComputeTrainPredictCategoricalFeature", CasePAIMaxComputeTrainPredictCategoricalFeature)
		t.Run("CasePAIMaxComputeTrainTFBTDistributed", CasePAIMaxComputeTrainTFBTDistributed)
		t.Run("CasePAIMaxComputeTrainDistributedKeras", CasePAIMaxComputeTrainDistributedKeras)
		t.Run("CasePAIMaxComputeTrainXGBDistributed", CasePAIMaxComputeTrainXGBDistributed)

		// FIXME(typhoonzero): Add this test back when we solve error: model already exist issue on the CI.
		// t.Run("CaseTrainPAIRandomForests", CaseTrainPAIRandomForests)
	})
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
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}
	t.Run("CaseWorkflowTrainAndPredictDNN", CaseWorkflowTrainAndPredictDNN)
}

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
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)

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

SELECT *
FROM %s
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM %s"
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;

SELECT *
FROM %s
TO PREDICT %s.class
USING %s;

SELECT *
FROM %s LIMIT 5;
	`, caseTrainTable, caseTrainTable, caseTestTable, caseInto, caseTestTable, casePredictTable, caseInto, casePredictTable)

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

func CaseShowTrain(t *testing.T) {
	driverName, _, _ := database.ParseURL(dbConnStr)
	if driverName != "mysql" && driverName != "hive" {
		t.Skip("Skipping non mysql/hive test.")
	}
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train TO TRAIN xgboost.gbtree
	WITH objective="reg:squarederror"
	LABEL class 
	INTO sqlflow_models.my_xgb_model_for_show_train;`
	_, _, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Train model failed: %v", err)
	}
	showSQL := `SHOW TRAIN sqlflow_models.my_xgb_model_for_show_train;`
	cols, _, _, err := connectAndRunSQL(showSQL)
	a.NoError(err)
	a.Equal(2, len(cols))
	a.Equal("Table", cols[0])
	a.Equal("Train Statement", cols[1])
}
