package main

import (
	"context"
	"log"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	a := assert.New(t)
	tests := []string{
		"show databases;",
		"select * from iris.train limit 2;",
	}

	go main()
	// FIXME(weiguo): We may need to expand sleep time
	time.Sleep(2 * time.Second)

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		_, err := cli.Run(ctx, &pb.Request{Sql: tc})
		if err != nil {
			log.Fatalf("Check if the server started successfully. %v", err)
		}
	}
}
