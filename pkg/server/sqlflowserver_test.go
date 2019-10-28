// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "sqlflow.org/sqlflow/pkg/server/proto"
	sf "sqlflow.org/sqlflow/pkg/sql"
)

const (
	testErrorSQL               = "ERROR ..."
	testQuerySQL               = "SELECT * FROM some_table;"
	testExecuteSQL             = "INSERT INTO some_table VALUES (1,2,3,4);"
	testExtendedSQL            = "SELECT * FROM some_table TRAIN SomeModel;"
	testExtendedSQLNoSemicolon = "SELECT * FROM some_table TRAIN SomeModel"
	testExtendedSQLWithSpace   = "SELECT * FROM some_table TRAIN SomeModel; \n\t"
)

var testServerAddress string

func mockRun(sql string, db *sf.DB, modelDir string, session *pb.Session) *sf.PipeReader {
	rd, wr := sf.Pipe()
	go func() {
		defer wr.Close()
		switch sql {
		case testErrorSQL:
			wr.Write(fmt.Errorf("run error: %v", testErrorSQL))
		case testQuerySQL:
			m := make(map[string]interface{})
			m["columnNames"] = []string{"X", "Y"}
			wr.Write(m)
			wr.Write([]interface{}{true, false, "hello", []byte("world")})
			wr.Write([]interface{}{int8(1), int16(1), int32(1), int(1), int64(1)})
			wr.Write([]interface{}{uint8(1), uint16(1), uint32(1), uint(1), uint64(1)})
			wr.Write([]interface{}{float32(1), float64(1)})
			wr.Write([]interface{}{time.Now(), nil})
		case testExecuteSQL:
			wr.Write("success; 0 rows affected")
		case testExtendedSQL, testExtendedSQLNoSemicolon, testExtendedSQLWithSpace:
			wr.Write("log 0")
			wr.Write("log 1")
		default:
			wr.Write(fmt.Errorf("unexcepted SQL: %s", sql))
		}
	}()
	return rd
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
	pb.RegisterSQLFlowServer(s, &Server{run: mockRun, db: nil})
	reflection.Register(s)
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func createRudeClient() {
	conn, _ := grpc.Dial(testServerAddress, grpc.WithInsecure())
	c := pb.NewSQLFlowClient(conn)
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.Run(ctx, &pb.Request{Sql: testQuerySQL})
	if err != nil {
		log.Fatalf("Run encounts err:%v", err)
	}

	// conn closed without *any* stream.Recv(), act as rude client
	conn.Close()
}

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
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
	stream, err := c.Run(ctx, &pb.Request{Sql: testErrorSQL, Session: &pb.Session{DbConnStr: dbConnStr}})
	a.NoError(err)
	_, err = stream.Recv()
	a.Equal(status.Error(codes.Unknown, "Lex: Unknown problem ..."), err)

	testMultipleSQL := fmt.Sprintf("%s %s", testQuerySQL, testExtendedSQL)
	for _, s := range []string{testQuerySQL, testExecuteSQL, testExtendedSQL, testExtendedSQLWithSpace, testExtendedSQLNoSemicolon, testMultipleSQL} {
		stream, err := c.Run(ctx, &pb.Request{Sql: s, Session: &pb.Session{DbConnStr: dbConnStr}})
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

func TestGoroutineLeaky(t *testing.T) {
	defer leaktest.CheckTimeout(t, 10*time.Second)()
	for i := 0; i < 50; i++ {
		go func() {
			createRudeClient()
		}()
	}
}

func TestMain(m *testing.M) {
	done := make(chan bool)
	go startServer(done)
	<-done

	os.Exit(m.Run())
}
