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
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/proto"
	pb "sqlflow.org/sqlflow/go/proto"
	sf "sqlflow.org/sqlflow/go/sql"
	server "sqlflow.org/sqlflow/go/sqlflowserver"
	"sqlflow.org/sqlflow/go/sqlfs"
	"sqlflow.org/sqlflow/go/tar"
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

func releaseDemoModelRepo(client proto.ModelZooServerClient) error {
	dir, err := mockTmpModelRepo()
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	if err := tar.ZipDir(".", "modelrepo.tar.gz"); err != nil {
		return err
	}
	buf, err := ioutil.ReadFile("modelrepo.tar.gz")
	if err != nil {
		return err
	}

	stream, err := client.ReleaseModelRepo(context.Background())
	if err != nil {
		return err
	}

	// release model repo with no content files will skip build and push docker image
	modelDefReq := &pb.ReleaseModelRepoRequest{
		Name:       "sqlflow/sqlflow",
		Tag:        "modelzootest",
		ContentTar: buf}
	err = stream.Send(modelDefReq)
	if err != nil {
		return err
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return err
	}
	return os.Chdir(cwd)
}

func TestUsingModelZooModel(t *testing.T) {
	// FIXME(sneaxiy): run this test when SQLFLOW_USE_EXPERIMENTAL_CODEGEN=true
	oldEnv := os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN")
	os.Setenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN", "")
	defer os.Setenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN", oldEnv)

	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("Skipping mysql tests")
	}
	os.Setenv("SQLFLOW_MODEL_ZOO_REGISTRY", "hub.docker.com")
	// start sqlflow server
	go startSqlflowServer()
	server.WaitPortReady("localhost:50052", 0)
	// start model zoo server
	go StartModelZooServer(50056, database.GetTestingMySQLURL())
	server.WaitPortReady("localhost:50056", 0)

	conn, err := grpc.Dial(":50056", grpc.WithInsecure())
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

	// read trained model in sqlflow_models.modelzoo_model_iris
	db, err := database.OpenAndConnectDB(database.GetTestingMySQLURL())
	a.NoError(err)
	defer db.Close()
	sqlf, err := sqlfs.Open(db.DB, "sqlflow_models.modelzoo_model_iris")
	a.NoError(err)
	defer sqlf.Close()
	// Note that modelBuf is a gob encoded struct
	var modelBuf bytes.Buffer
	_, err = modelBuf.ReadFrom(sqlf)
	a.NoError(err)
	// release the model repo "sqlflow/sqlflow:modelzootest" beforehand
	err = releaseDemoModelRepo(modelZooClient)
	a.NoError(err)

	req := &proto.ReleaseModelRequest{
		Name:        "sqlflow_models.modelzoo_model_iris",
		Tag:         "v0.1",
		Description: "a test release model trained by iris dataset",
		DbConnStr:   database.GetTestingMySQLURL(),
	}
	_, err = modelZooClient.ReleaseModel(context.Background(), req)
	a.NoError(err)

	err = execStmt(sqlflowServerClient, `SELECT * FROM iris.train
TO PREDICT iris.modelzoo_predict.class
USING localhost:50056/sqlflow_models.modelzoo_model_iris:v0.1;`)
	a.NoError(err)

	_, err = modelZooClient.DropModel(context.Background(), &proto.ReleaseModelRequest{
		Name: "sqlflow_models.modelzoo_model_iris",
		Tag:  "v0.1",
	})
	a.NoError(err)

	_, err = modelZooClient.DropModelRepo(context.Background(), &proto.ReleaseModelRepoRequest{
		Name: "sqlflow/sqlflow",
		Tag:  "modelzootest",
	})
	a.NoError(err)
}
