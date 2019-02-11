package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	testQuerySQL    = "SELECT ..."
	testExecuteSQL  = "INSERT ..."
	testExtendedSQL = "SELECT ... TRAIN ..."
)

var testServerAddress string

func TestSQL(t *testing.T) {
	a := assert.New(t)
	// Set up a connection to the server.
	conn, err := grpc.Dial(testServerAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewSQLFlowClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	queryStream, err := c.Query(ctx, &pb.Request{Sql: testQuerySQL})
	a.NoError(err)
	for {
		_, err := queryStream.Recv()
		if err == io.EOF {
			break
		}
		a.NoError(err)
	}

	for _, sql := range []string{testExecuteSQL, testExtendedSQL} {
		executeStream, err := c.Execute(ctx, &pb.Request{Sql: sql})
		a.NoError(err)
		for {
			_, err := executeStream.Recv()
			if err == io.EOF {
				break
			}
			a.NoError(err)
		}
	}
}

func startServer(done chan bool) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	testServerAddress = fmt.Sprintf("localhost:%v", listener.Addr().(*net.TCPAddr).Port)
	done <- true

	s := grpc.NewServer()
	s.GetServiceInfo()
	pb.RegisterSQLFlowServer(s, &Server{})
	reflection.Register(s)
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func TestMain(m *testing.M) {
	done := make(chan bool)
	go startServer(done)
	<-done

	os.Exit(m.Run())
}
