package main

import (
	"context"
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

func TestEnd2EndFlow(t *testing.T) {
	go start("mysql://root:root@tcp/?maxAllowedPacket=0")
	WaitPortReady("localhost"+port, 0)
	t.Run("TestStandardSQL", CaseStandardSQL)
	t.Run("TestTrainSQL", CaseTrainSQL)
}

func CaseStandardSQL(t *testing.T) {
	a := assert.New(t)
	tests := []string{
		"show databases;",
		"select * from iris.train limit 2;",
	}

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		stream, err := cli.Run(ctx, &pb.Request{Sql: tc})
		if err != nil {
			a.Fail("Check if the server started successfully. %v", err)
		}
		for {
			iter, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("stream read err: %v", err)
			}

			switch x := iter.Response.(type) {
			case *pb.Response_Head:
				log.Println(x.Head)
			case *pb.Response_Message:
				log.Println(x.Message)
			case *pb.Response_Row:
				row := x.Row
				log.Println(row.Data)
			default:
				log.Printf("Response have unexpected type: %T", x)
			}

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
