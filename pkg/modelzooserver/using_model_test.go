// Copyright 2020 The SQLFlow Authors. All rights reserved.
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

package modelzooserver

import (
	"context"
	"io"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/server"
	sf "sqlflow.org/sqlflow/pkg/sql"
)

func startSqlflowServer() error {
	s := grpc.NewServer()
	proto.RegisterSQLFlowServer(s, server.NewServer(sf.RunSQLProgram, ""))
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		return err
	}
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func execStmt(client proto.SQLFlowClient, sql string) error {
	req := &proto.Request{
		Sql: sql,
		Session: &proto.Session{
			Token:     "user-unittest",
			DbConnStr: database.GetTestingMySQLURL(),
		}}

	stream, err := client.Run(context.Background(), req)
	if err != nil {
		return err
	}

	// fetch stream to wait the statement execution finish
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func TestUsingModelZooModel(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	// start sqlflow server
	go startSqlflowServer()
	server.WaitPortReady("localhost:50052", 0)
	// start model zoo server
	go startServer()
	server.WaitPortReady("localhost:50055", 0)

	conn, err := grpc.Dial(":50055", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("create client error: %v", err)
	}
	defer conn.Close()
	modelZooClient := proto.NewModelZooServerClient(conn)

	conn2, err := grpc.Dial(":50052", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("create client error: %v", err)
	}
	defer conn2.Close()
	sqlflowServerClient := proto.NewSQLFlowClient(conn2)

	// 1. train a model
	// 2. release the trained model
	// 3. predict using the released model in model zoo
	a := assert.New(t)

	err = execStmt(sqlflowServerClient, `SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH
	model.n_classes = 3,
	model.hidden_units = [10, 20],
	validation.select = "SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.modelzoo_model_iris;`)
	a.NoError(err)

	stream, err := modelZooClient.ReleaseModel(context.Background())
	a.NoError(err)
	err = stream.Send(&proto.ReleaseModelRequest{
		Name:                    "modelzoo_model_iris",
		Tag:                     "v0.1",
		Description:             "a test release model trained by iris dataset",
		EvaluationMetrics:       "", // TODO(typhoonzero): need to support find metrics in the trained model
		ModelClassName:          "DNNClassifier",
		ModelCollectionImageUrl: "sqlflow/sqlflow",
		ContentTar:              []byte{},
		ContentUrl:              "", // not used
	})
	a.NoError(err)
	_, err = stream.CloseAndRecv()

	err = execStmt(sqlflowServerClient, `SELECT * FROM iris.train
TO PREDICT iris.modelzoo_predict.class
USING localhost:50055/modelzoo_model_iris;`)
	a.NoError(err)
}
