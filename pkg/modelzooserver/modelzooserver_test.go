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
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/server"
)

func startServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 50055))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	mysqlConn, err := database.OpenAndConnectDB(database.GetTestingMySQLURL())
	if err != nil {
		log.Fatalf("failed to connect to mysql: %v", err)
	}
	splitedStmts := strings.Split(createTableStmts, ";")
	for idx, stmt := range splitedStmts {
		if idx == len(splitedStmts)-1 {
			// the last stmt is empty
			break
		}
		_, err = mysqlConn.Exec(stmt)
		if err != nil {
			log.Fatalf("failed to create model zoo tables: %v", err)
		}
	}

	pb.RegisterModelZooServerServer(grpcServer, &modelZooServer{DB: mysqlConn})

	grpcServer.Serve(lis)
}

func mockTmpModelRepo() (string, error) {
	dir, err := ioutil.TempDir("/tmp", "tmp-sqlflow-repo")
	if err != nil {
		return "", err
	}
	modelRepoDir := fmt.Sprintf("%s/my_test_models", dir)
	if err := os.Mkdir(modelRepoDir, os.ModeDir); err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(
		fmt.Sprintf("%s/Dockerfile", dir), []byte(sampleDockerfile), 0644); err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(
		fmt.Sprintf("%s/my_test_model.py", modelRepoDir),
		[]byte(sampleModelCode), 0644); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(
		fmt.Sprintf("%s/__init__.py", modelRepoDir),
		[]byte(sampleInitCode), 0644); err != nil {
		return "", err
	}

	return dir, nil
}

func TestModelZooServer(t *testing.T) {
	a := assert.New(t)
	go startServer()
	server.WaitPortReady("localhost:50055", 0)

	conn, err := grpc.Dial(":50055", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("create client error: %v", err)
	}
	defer conn.Close()

	client := pb.NewModelZooServerClient(conn)

	dir, err := mockTmpModelRepo()
	a.NoError(err)
	defer os.RemoveAll(dir)
	cwd, err := os.Getwd()
	a.NoError(err)
	err = os.Chdir(dir)
	a.NoError(err)

	// tar the mocked files and do release
	err = tarGzDir("my_test_models", "modelrepo.tar.gz")
	a.NoError(err)
	stream, err := client.ReleaseModelDef(context.Background())
	a.NoError(err)
	buf, err := ioutil.ReadFile("modelrepo.tar.gz")
	a.NoError(err)
	modelDefReq := &pb.ModelDefRequest{
		Name:       "typhoon1986/my_test_model",
		Tag:        "v0.1",
		ContentTar: buf}
	err = stream.Send(modelDefReq)
	a.NoError(err)

	reply, err := stream.CloseAndRecv()
	a.NoError(err)
	a.Equal(true, reply.Success)

	err = os.Chdir(cwd)
	a.NoError(err)

	res, err := client.ListModelDefs(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
	a.NoError(err)
	a.Equal(1, len(res.ModelDefList))
	a.Equal("typhoon1986/my_test_model", res.ModelDefList[0].ImageUrl)
	a.Equal("DNNClassifier", res.ModelDefList[0].ClassName)
	a.Equal(307, len(res.ModelDefList[0].ArgDescs))

	trainedModelRes, err := client.ReleaseTrainedModel(context.Background(),
		&pb.TrainedModelRequest{
			Name:                    "my_regression_model",
			Tag:                     "v0.1",
			ContentUrl:              "oss://bucket/path/to/my/model",
			Description:             "A linear regression model for house price predicting",
			EvaluationMetrics:       "MSE: 0.02, MAPE: 10.32",
			ModelClassName:          "DNNClassifier",
			ModelCollectionImageUrl: "hub.docker.com/group/mymodel:v0.1",
		})
	a.NoError(err)
	a.Equal(true, trainedModelRes.Success)

	listTrainedModelRes, err := client.ListTrainedModels(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
	a.NoError(err)
	a.Equal(1, len(listTrainedModelRes.TrainedModelList))
	a.Equal("my_regression_model", listTrainedModelRes.TrainedModelList[0].Name)
	a.Equal("hub.docker.com/group/mymodel:v0.1", listTrainedModelRes.TrainedModelList[0].ImageUrl)

	_, err = client.DropTrainedModel(context.Background(), &pb.TrainedModelRequest{
		Name: "my_regression_model", Tag: "v0.1",
	})

	listTrainedModelRes, err = client.ListTrainedModels(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
	a.NoError(err)
	a.Equal(0, len(listTrainedModelRes.TrainedModelList))

	_, err = client.DropModelDef(context.Background(), modelDefReq)
	a.NoError(err)

	res, err = client.ListModelDefs(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
	a.NoError(err)
	a.Equal(0, len(res.ModelDefList))
}
