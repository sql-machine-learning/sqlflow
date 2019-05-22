package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"github.com/stretchr/testify/assert"
)

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

func TestEnd2EndMySQL(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	// default run mysql tests
	if len(testDBDriver) == 0 {
		testDBDriver = "mysql"
	}
	if testDBDriver != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	go start("mysql://root:root@tcp/?maxAllowedPacket=0")
	WaitPortReady("localhost"+port, 0)
	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
	t.Run("TestTextClassification", CaseTrainTextClassification)
}

func TestEnd2EndHive(t *testing.T) {
	testDBDriver := os.Getenv("SQLFLOW_TEST_DB")
	if testDBDriver != "hive" {
		t.Skip("Skipping hive tests")
	}
	go start("hive://127.0.0.1:10000/iris")
	WaitPortReady("localhost"+port, 0)
	t.Run("TestShowDatabases", CaseShowDatabases)
	t.Run("TestSelect", CaseSelect)
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func CaseShowDatabases(t *testing.T) {
	a := assert.New(t)
	cmd := "show databases;"

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: cmd})
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
		"toutiao":            "",
		"hive":               "", // if current mysql is also used for hive
		"default":            "", // if fetching default hive databases
	}
	for i := 0; i < len(resp); i++ {
		AssertContainsAny(a, expectedDBs, resp[i][0])
	}
}

func CaseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := "select * from iris.train limit 2;"

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: cmd})
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
	trainSQL := `SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;`

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)

	// FIXME(typhoonzero): Fix PREDICT tests using hive
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		return
	}

	predSQL := `SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`

	stream, err = cli.Run(ctx, &pb.Request{Sql: predSQL})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait predict finish
	ParseRow(stream)

	showPred := `SELECT *
FROM iris.predict LIMIT 5;`

	stream, err = cli.Run(ctx, &pb.Request{Sql: showPred})
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

// CaseTrainSQL is a simple End-to-End testing for case training and predicting
func CaseTrainCustomModel(t *testing.T) {
	a := assert.New(t)
	trainSQL := `SELECT *
FROM iris.train
TRAIN sqlflow_models.DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model_custom;`

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)

	// FIXME(typhoonzero): Fix PREDICT tests using hive
	if os.Getenv("SQLFLOW_TEST_DB") == "hive" {
		return
	}

	predSQL := `SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model_custom;`

	stream, err = cli.Run(ctx, &pb.Request{Sql: predSQL})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait predict finish
	ParseRow(stream)

	showPred := `SELECT *
FROM iris.predict LIMIT 5;`

	stream, err = cli.Run(ctx, &pb.Request{Sql: showPred})
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
FROM toutiao.train_processed
TRAIN DNNClassifier
WITH n_classes = 17, hidden_units = [10, 20]
COLUMN news_title
LABEL class_id
INTO sqlflow_models.my_dnn_model;`

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: trainSQL})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	// call ParseRow only to wait train finish
	ParseRow(stream)
}
