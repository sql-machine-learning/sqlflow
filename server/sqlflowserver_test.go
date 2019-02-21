package server

import (
	"context"
	"database/sql"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	testErrorSQL    = "ERROR ..."
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

	stream, err := c.Run(ctx, &pb.Request{Sql: testErrorSQL})
	a.NoError(err)
	_, err = stream.Recv()
	a.Equal(status.Error(codes.Unknown, fmt.Sprintf("run error: %v", testErrorSQL)), err)

	for _, s := range []string{testQuerySQL, testExecuteSQL, testExtendedSQL} {
		stream, err := c.Run(ctx, &pb.Request{Sql: s})
		a.NoError(err)
		for {
			rsp, err := stream.Recv()
			log.Fatalf("999999999999999\nrsp:%v\nerr:%v\n", rsp, err)
			if err == io.EOF {
				break
			}
			a.NoError(err)
		}
	}
}

func mockRun(sql string, db *sql.DB) chan interface{} {
	c := make(chan interface{})

	go func() {
		defer close(c)
		switch sql {
		case testErrorSQL:
			c <- fmt.Errorf("run error: %v", testErrorSQL)
		case testQuerySQL:
			m := make(map[string]interface{})
			m["columnNames"] = []string{"X", "Y"}
			c <- m
			c <- []interface{}{true, false, "hello", []byte("world")}
			c <- []interface{}{int8(1), int16(1), int32(1), int(1), int64(1)}
			c <- []interface{}{uint8(1), uint16(1), uint32(1), uint(1), uint64(1)}
			c <- []interface{}{float32(1), float64(1), time.Now()}
			// FIXME(weiguo): c <- []interface{}{nil}
		case testExecuteSQL:
			c <- "success; 0 rows affected"
		case testExtendedSQL:
			c <- "log 0"
			c <- "log 1"
			c <- "log 2"
			c <- "log 3"
		}
	}()
	return c
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
	pb.RegisterSQLFlowServer(s, &server{run: mockRun, db: nil})
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
