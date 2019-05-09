package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
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
			onerow := iter.GetRow()
			rows = append(rows, onerow.Data)
		}
		counter++
	}
	return columns, rows
}

func TestEnd2EndFlow(t *testing.T) {
	go start("mysql://root:root@tcp/?maxAllowedPacket=0")
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: cmd})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	head, resp := ParseRow(stream)
	a.Equal("Database", head[0])

	expectedDBs := []string{
		"information_schema",
		"churn",
		"iris",
		"mysql",
		"performance_schema",
		"sqlflow_models",
		"sqlfs_test",
		"sys",
	}
	for i := 0; i < len(resp); i++ {
		a.Equal(expectedDBs[i], string(resp[i][0].Value[2:]))
	}
}

func CaseSelect(t *testing.T) {
	a := assert.New(t)
	cmd := "select * from iris.train limit 2;"

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	for i := 0; i < len(head); i++ {
		a.Equal(expectedHeads[i], head[i])
	}
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(rows[i]); j++ {
			var pb proto.Message
			ptypes.UnmarshalAny(rows[i][j], pb)
			fmt.Println(pb)
		}
	}

}

func CaseTrainSQL(t *testing.T) {
	a := assert.New(t)
	sql := `SELECT *
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Run(ctx, &pb.Request{Sql: sql})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
}
