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

func connectAndRunSQL(sql string) ([]string, [][]*any.Any, error) {
	conn, err := createRPCConn()
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()
	stream, err := cli.Run(ctx, sqlRequest(sql))
	if err != nil {
		return nil, nil, err
	}
	cols, rows := ParseRow(stream)
	return cols, rows, nil
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
	testDB, e := database.OpenAndConnectDB(dbStr)
	if e != nil {
		return e
	}

	db := os.Getenv("SQLFLOW_TEST_DB")
	if db != "maxcompute" {
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
	case "maxcompute":
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

	go start(modelDir, caCrt, caKey, unitTestPort, false)
	waitPortReady(fmt.Sprintf("localhost:%d", unitTestPort), 0)
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainBoostedTreesEstimatorAndExplain", CaseTrainBoostedTreesEstimatorAndExplain)
	t.Run("CaseTrainSQLWithMetrics", CaseTrainSQLWithMetrics)
	t.Run("TestTextClassification", CaseTrainTextClassification)
	t.Run("CaseTrainTextClassificationCustomLSTM", CaseTrainTextClassificationCustomLSTM)
	t.Run("CaseTrainCustomModel", CaseTrainCustomModel)
	t.Run("CaseTrainOptimizer", CaseTrainOptimizer)
	t.Run("CaseTrainSQLWithHyperParams", CaseTrainSQLWithHyperParams)
	t.Run("CaseTrainCustomModelWithHyperParams", CaseTrainCustomModelWithHyperParams)
	t.Run("CaseSparseFeature", CaseSparseFeature)
	t.Run("CaseSQLByPassLeftJoin", CaseSQLByPassLeftJoin)
	t.Run("CaseTrainRegression", CaseTrainRegression)
	t.Run("CaseTrainXGBoostRegression", CaseTrainXGBoostRegression)
	t.Run("CasePredictXGBoostRegression", CasePredictXGBoostRegression)
	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
	t.Run("CaseTrainDeepWideModelOptimizer", CaseTrainDeepWideModelOptimizer)

	// Cases using feature derivation
	t.Run("CaseTrainTextClassificationIR", CaseTrainTextClassificationIR)
	t.Run("CaseTrainTextClassificationFeatureDerivation", CaseTrainTextClassificationFeatureDerivation)
	t.Run("CaseXgboostFeatureDerivation", CaseXgboostFeatureDerivation)
	t.Run("CaseTrainFeatureDerivation", CaseTrainFeatureDerivation)
}

func CaseXgboostFeatureDerivation(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM housing.train
TO TRAIN xgboost.gbtree
WITH objective="reg:squarederror",
	 train.num_boost_round=30
LABEL target
INTO sqlflow_models.my_xgb_regression_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run test error: %v", err)
	}

	predSQL := `SELECT * FROM housing.test
TO PREDICT housing.predict.target
USING sqlflow_models.my_xgb_regression_model;`
	_, _, err = connectAndRunSQL(predSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
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
	t.Run("CaseTrainAdaNet", CaseTrainAdaNet)
	t.Run("CaseTrainOptimizer", CaseTrainOptimizer)
	t.Run("CaseTrainDeepWideModel", CaseTrainDeepWideModel)
	t.Run("CaseTrainDeepWideModelOptimizer", CaseTrainDeepWideModelOptimizer)
	t.Run("CaseTrainXGBoostRegression", CaseTrainXGBoostRegression)
	t.Run("CasePredictXGBoostRegression", CasePredictXGBoostRegression)
	t.Run("CaseTrainFeatureDerivation", CaseTrainFeatureDerivation)
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
// 	_, _, err := connectAndRunSQL(trainSQL)
// 	if err != nil {
// 		a.Fail("run trainSQL error: %v", err)
// 	}
// }

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
	trainSQL := fmt.Sprintf(`
	SELECT *
	FROM %s
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 3,
		model.hidden_units = [10, 20],
		validation.select = "SELECT * FROM %s LIMIT 30"
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseTrainTable, caseInto)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	predSQL := fmt.Sprintf(`SELECT *
FROM %s
TO PREDICT %s.class
USING %s;`, caseTestTable, casePredictTable, caseInto)
	_, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

	showPred := fmt.Sprintf(`SELECT *
FROM %s LIMIT 5;`, casePredictTable)
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
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}

	explainSQL := fmt.Sprintf(`SELECT * FROM iris.test WHERE class!=2
	TO EXPLAIN %s
	INTO iris.explain_result;`, caseInto)
	_, _, err = connectAndRunSQL(explainSQL)
	a.NoError(err)

	getExplainResult := `SELECT * FROM iris.explain_result;`
	_, rows, err := connectAndRunSQL(getExplainResult)
	a.NoError(err)
	for _, row := range rows {
		AssertGreaterEqualAny(a, row[1], float32(0))
	}
}

func CaseTrainLinearClassifier(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`SELECT * FROM iris.train WHERE class !=2
TO TRAIN LinearClassifier LABEL class INTO %s;`, caseInto)
	_, _, err := connectAndRunSQL(trainSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
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
	_, _, err = connectAndRunSQL(kerasTrainSQL)
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
	_, _, err = connectAndRunSQL(regressionTrainSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)

	predSQL := `SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`
	_, _, err = connectAndRunSQL(predSQL)
	a.NoError(err)

	// TODO(typhoonzero): also support string column type for training and prediction (column c6)
	// NOTE(typhoonzero): this test also tests saving to the same model name when saving to model zoo table (sqlflow.trained_models)
	trainVaryColumnTypes := `SELECT c1, c2, c3, c4, c5, class from feature_derivation_case.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,10]
COLUMN EMBEDDING(c3, 32, sum), EMBEDDING(SPARSE(c5, 64, COMMA), 32, sum)
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, err = connectAndRunSQL(trainVaryColumnTypes)
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
	_, _, err := connectAndRunSQL(trainSQL)
	a.NoError(err)

	predSQL := `SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`
	_, _, err = connectAndRunSQL(predSQL)
	a.NoError(err)

	trainKerasSQL := `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20],
	 model.optimizer=RMSprop, optimizer.learning_rate=0.1,
	 model.loss=SparseCategoricalCrossentropy
LABEL class
INTO sqlflow_models.my_dnn_model;`
	_, _, err = connectAndRunSQL(trainKerasSQL)
	a.NoError(err)
}

func CaseTrainCustomModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
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
TO PREDICT iris.predict.class
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

	trainSQL = `SELECT *
FROM iris.train
TO TRAIN sqlflow_models.DNNClassifier
WITH model.n_classes = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom_functional;`
	_, _, err = connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
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
	_, _, err := connectAndRunSQL(trainSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
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
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
}

func CaseTrainAdaNet(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT * FROM iris.train
TO TRAIN sqlflow_models.AutoClassifier WITH model.n_classes = 3
LABEL class
INTO sqlflow_models.my_adanet_model;`
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
	}
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
TO TRAIN sqlflow_models.DNNClassifier
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
	trainSQL := `SELECT news_title, class_id
FROM text_cn.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN EMBEDDING(SPARSE(news_title,16000,COMMA),128,mean)
LABEL class_id
INTO sqlflow_models.my_dnn_model;`
	_, _, err := connectAndRunSQL(trainSQL)
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
TO TRAIN alipay.SoftmaxClassifier
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
TO TRAIN LinearRegressor
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
TO PREDICT housing.predict.result
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
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	train.num_boost_round = 30,
	validation.select="SELECT * FROM housing.train LIMIT 20"
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
`)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("run trainSQL error: %v", err)
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
	train.num_boost_round = 30
	COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;
	`
	explainStmt := `
SELECT *
FROM housing.train
TO EXPLAIN sqlflow_models.my_xgb_regression_model
WITH
    shap_summary.plot_type="bar",
    shap_summary.alpha=1,
    shap_summary.sort=True
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
	ParseRow(stream)
	stream, err = cli.Run(ctx, sqlRequest(explainStmt))
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	ParseRow(stream)
}

func CasePredictXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	predSQL := fmt.Sprintf(`SELECT *
FROM housing.test
TO PREDICT housing.xgb_predict.target
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

func CaseTrainDistributedPAI(t *testing.T) {
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
		train.epoch=10,
		train.batch_size=4,
		train.verbose=2
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;
	`, caseTrainTable, caseInto)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
	predSQL := fmt.Sprintf(`SELECT *
FROM %ss
TO PREDICT %s.class
USING %s;`, caseTestTable, casePredictTable, caseInto)
	_, _, err = connectAndRunSQL(predSQL)
	if err != nil {
		a.Fail("Run predSQL error: %v", err)
	}

}

func CaseTrainPAIRandomForests(t *testing.T) {
	a := assert.New(t)
	trainSQL := fmt.Sprintf(`
	SELECT * FROM %s.%s
	TO TRAIN randomforests
	WITH tree_num = 3
	LABEL class
	INTO my_rf_model;
	`, caseDB, caseTrainTable)
	_, _, err := connectAndRunSQL(trainSQL)
	if err != nil {
		a.Fail("Run trainSQL error: %v", err)
	}
}

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
	err = prepareTestData(dbConnStr)
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

	t.Run("CaseTrainSQL", CaseTrainSQL)
	t.Run("CaseTrainPAIRandomForests", CaseTrainPAIRandomForests)
	t.Run("CaseTrainDistributedPAI", CaseTrainDistributedPAI)
}

func TestEnd2EndWorkflow(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DATASOURCE") == "" || strings.ToLower(os.Getenv("SQLFLOW_TEST")) != "workflow" {
		t.Skip("Skipping workflow test.")
	}
	driverName, _, err := database.ParseURL(testDatasource)
	a.NoError(err)

	if driverName != "mysql" && driverName != "maxcompute" {
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
	if err != nil {
		t.Fatalf("prepare test dataset failed: %v", err)
	}

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

		err = prepareTestData(dbConnStr)
		if err != nil {
			t.Fatalf("prepare test dataset failed: %v", err)
		}
	}

	t.Run("CaseWorkflowTrainAndPredictDNN", CaseWorkflowTrainAndPredictDNN)
	t.Run("CaseTrainDistributedPAIArgo", CaseTrainDistributedPAIArgo)
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
	model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO %s;

SELECT *
FROM %s
TO PREDICT %s.class
USING %s;

SELECT *
FROM %s LIMIT 5;
	`, caseTrainTable, caseTrainTable, caseInto, caseTestTable, casePredictTable, caseInto, casePredictTable)

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
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
	if !strings.HasPrefix(workflowID, "sqlflow-couler") {
		return fmt.Errorf("workflow not started with sqlflow-couler")
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
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("workflow times out")
}

func CaseTrainDistributedPAIArgo(t *testing.T) {
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
		train.verbose=2
	COLUMN sepal_length, sepal_width, petal_length, petal_width
	LABEL class
	INTO %s;

	SELECT *
FROM %s
TO PREDICT %s.class
USING %s;
	`, caseTrainTable, caseInto, caseTestTable, casePredictTable, caseInto)

	conn, err := createRPCConn()
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	defer conn.Close()

	cli := pb.NewSQLFlowClient(conn)
	// wait 30min for the workflow execution since it may take time to allocate enough nodes.
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL, Session: &pb.Session{DbConnStr: testDatasource}})
	if err != nil {
		a.Fail("Create gRPC client error: %v", err)
	}
	a.NoError(checkWorkflow(ctx, cli, stream))
}
