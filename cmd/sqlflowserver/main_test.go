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

func ParseRow(stream pb.SQLFlow_RunClient) ([]string, []string) {
	var resp []string
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
			row := iter.GetRow()
			for i := 0; i < len(row.Data); i++ {
				resp = append(resp, string(row.Data[i].Value))
			}
		}
		counter++
	}
	return columns, resp
}

func TestEnd2EndFlow(t *testing.T) {
	go start("mysql://root:root@tcp/?maxAllowedPacket=0")
	WaitPortReady("localhost"+port, 0)
	t.Run("TestStandardSQL", CaseStandardSQL)
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func CaseStandardSQL(t *testing.T) {
	a := assert.New(t)
	// tests := []string{
	// 	"show databases;",
	// 	"select * from iris.train limit 2;",
	// }

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := cli.Run(ctx, &pb.Request{Sql: "show databases;"})
	if err != nil {
		a.Fail("Check if the server started successfully. %v", err)
	}
	head, resp := ParseRow(stream)
	a.Equal("Database", head[0])
	for i := 0; i < len(resp); i++ {
		fmt.Println(string(resp[i]))
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
