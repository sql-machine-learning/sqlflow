package main

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/stretchr/testify/assert"
	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
)

func TestRun(t *testing.T) {
	a := assert.New(t)
	tests := []string{
		"show databases",
		"select * from iris.iris limit 2",
	}

	go main()
	time.Sleep(2 * time.Second)

	conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
	a.NoError(err)
	defer conn.Close()
	cli := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		_, err := cli.Run(ctx, &pb.Request{Sql: tc})
		a.NoError(err)
	}
}
