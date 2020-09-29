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
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sqlflow.org/sqlflow/go/database"
	sqlflowlog "sqlflow.org/sqlflow/go/log"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql/testdata"
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

const unitTestPort = 50061

func init() {
	sqlflowlog.InitLogger("/dev/null", sqlflowlog.TextFormatter)
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
	return ParseResponse(stream)
}

func sqlRequest(sql string) *pb.Request {

	se := &pb.Session{
		Token:     "user-unittest",
		DbConnStr: dbConnStr,
	}
	return &pb.Request{Sql: sql, Session: se}
}

// EqualAny checks any type of returned protobuf message to an interface
func EqualAny(expected interface{}, actual *any.Any) bool {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.StringValue":
		b := wrappers.StringValue{}
		ptypes.UnmarshalAny(actual, &b)
		return expected == b.Value
	case "type.googleapis.com/google.protobuf.FloatValue":
		b := wrappers.FloatValue{}
		ptypes.UnmarshalAny(actual, &b)
		return math.Abs(expected.(float64)-float64(b.Value)) < 1e-7
	case "type.googleapis.com/google.protobuf.DoubleValue":
		b := wrappers.DoubleValue{}
		ptypes.UnmarshalAny(actual, &b)
		return math.Abs(expected.(float64)-b.Value) < 1e-7
	case "type.googleapis.com/google.protobuf.Int64Value":
		b := wrappers.Int64Value{}
		ptypes.UnmarshalAny(actual, &b)
		return expected.(int64) == b.Value
	case "type.googleapis.com/google.protobuf.Int32Value":
		b := wrappers.Int32Value{}
		ptypes.UnmarshalAny(actual, &b)
		// convert expected to int32 value to compare
		v, ok := expected.(int32)
		if !ok {
			v64, ok := expected.(int64)
			if !ok {
				return false
			}
			v = int32(v64)
		}
		return v == b.Value
	}
	return false
}

// AssertGreaterEqualAny checks the protobuf value is greater than expected value.
func AssertGreaterEqualAny(a *assert.Assertions, actual *any.Any, expected interface{}) {
	switch actual.TypeUrl {
	case "type.googleapis.com/google.protobuf.Int64Value":
		b := wrappers.Int64Value{}
		ptypes.UnmarshalAny(actual, &b)
		a.GreaterOrEqual(b.Value, expected.(int64))
	case "type.googleapis.com/google.protobuf.FloatValue":
		b := wrappers.FloatValue{}
		ptypes.UnmarshalAny(actual, &b)
		if f64, ok := expected.(float64); ok {
			a.GreaterOrEqual(b.Value, float32(f64))
		} else {
			a.GreaterOrEqual(b.Value, expected.(float32))
		}
	case "type.googleapis.com/google.protobuf.DoubleValue":
		b := wrappers.DoubleValue{}
		ptypes.UnmarshalAny(actual, &b)
		if f64, ok := expected.(float64); ok {
			a.GreaterOrEqual(b.Value, float64(float32(f64)))
		} else {
			a.GreaterOrEqual(b.Value, float64(expected.(float32)))
		}
	default:
		a.Fail(fmt.Sprintf("unsupported type comparison %v %T", actual.TypeUrl, expected))
	}
}

// AssertContainsAny checks the protobuf value contains in all
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

// AssertIsSubStringAny assert the protobuf message contains substring
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

// ParseResponse parse grpc server stream response
func ParseResponse(stream pb.SQLFlow_RunClient) ([]string, [][]*any.Any, []string, error) {
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
			return nil, nil, nil, err
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
	return columns, rows, messages, nil
}

func prepareTestData(dbStr string) error {
	testDB, e := database.OpenAndConnectDB(dbStr)
	if e != nil {
		return e
	}
	defer testDB.Close()

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
			testdata.TextCNSQL,
			testdata.FundSQL,
			testdata.OptimizeCaseSQL,
			testdata.XGBoostSparseDataCaseSQL}
		datasets = append(datasets, fmt.Sprintf(testdata.WeightedKeyValueCaseSQL, caseDB))
	case "hive":
		datasets = []string{
			testdata.IrisHiveSQL,
			testdata.ChurnHiveSQL,
			testdata.FeatureDerivationCaseSQLHive,
			testdata.HousingSQL,
			testdata.OptimizeCaseSQL,
			testdata.XGBoostHiveSparseDataCaseSQL}
		datasets = append(datasets, fmt.Sprintf(testdata.WeightedKeyValueCaseSQLHive, caseDB))
	case "maxcompute", "alisa":
		if os.Getenv("SQLFLOW_submitter") == "alps" {
			datasets = []string{
				testdata.ODPSFeatureMapSQL,
				testdata.ODPSSparseColumnSQL,
			}
		}

		datasets = append(datasets,
			fmt.Sprintf(testdata.IrisMaxComputeSQL, caseDB),
			fmt.Sprintf(testdata.ChurnMaxComputeSQL, caseDB),
			fmt.Sprintf(testdata.XGBoostMaxComputeSparseDataCaseSQL, caseDB),
			fmt.Sprintf(testdata.WeightedKeyValueCaseSQLMaxCompute, caseDB),
			fmt.Sprintf(testdata.FeatureDerivationCaseSQLMaxCompute, caseDB))
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
