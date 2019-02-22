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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
	sf "gitlab.alipay-inc.com/Arc/sqlflow/sql"
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
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			a.NoError(err)
		}
	}
}

func mockRun(sql string, db *sql.DB) *sf.ExecutorChan {
	rsp := sf.NewExecutorChan()
	go func() {
		defer rsp.Destroy()

		switch sql {
		case testErrorSQL:
			rsp.Write(fmt.Errorf("run error: %v", testErrorSQL))
		case testQuerySQL:
			m := make(map[string]interface{})
			m["columnNames"] = []string{"X", "Y"}
			rsp.Write(m)
			rsp.Write([]interface{}{true, false, "hello", []byte("world")})
			rsp.Write([]interface{}{int8(1), int16(1), int32(1), int(1), int64(1)})
			rsp.Write([]interface{}{uint8(1), uint16(1), uint32(1), uint(1), uint64(1)})
			rsp.Write([]interface{}{float32(1), float64(1)})
			rsp.Write([]interface{}{time.Now(), nil})
		case testExecuteSQL:
			rsp.Write("success; 0 rows affected")
		case testExtendedSQL:
			rsp.Write("log 0")
			rsp.Write("log 1")
		}
	}()
	return rsp
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
