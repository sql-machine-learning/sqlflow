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
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	server "sqlflow.org/sqlflow/pkg/sqlflowserver"
	"sqlflow.org/sqlflow/pkg/tar"
)

func mockTmpModelRepo() (string, error) {
	dir, err := ioutil.TempDir("/tmp", "tmp-sqlflow-repo")
	if err != nil {
		return "", err
	}
	modelRepoDir := fmt.Sprintf("%s/my_test_models", dir)
	if err := os.Mkdir(modelRepoDir, 0755); err != nil {
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
	go StartModelZooServer(50055, database.GetTestingMySQLURL())
	server.WaitPortReady("localhost:50055", 0)

	conn, err := grpc.Dial(":50055", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("create client error: %v", err)
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)

	t.Run("ReleaseModelDef", func(t *testing.T) {
		dir, err := mockTmpModelRepo()
		a.NoError(err)
		defer os.RemoveAll(dir)
		cwd, err := os.Getwd()
		a.NoError(err)
		err = os.Chdir(dir)
		a.NoError(err)

		err = tar.ZipDir(".", "modelrepo.tar.gz")
		a.NoError(err)
		stream, err := client.ReleaseModelRepo(context.Background())
		a.NoError(err)
		buf, err := ioutil.ReadFile("modelrepo.tar.gz")
		a.NoError(err)
		modelDefReq := &pb.ReleaseModelRepoRequest{
			Name:       "sqlflow/my_test_model",
			Tag:        "v0.1",
			ContentTar: buf}
		err = stream.Send(modelDefReq)
		a.NoError(err)

		reply, err := stream.CloseAndRecv()
		if err != nil {
			a.FailNow("%v", err)
		}
		a.Equal(true, reply.Success)

		err = os.Chdir(cwd)
		a.NoError(err)

		res, err := client.ListModelRepos(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
		a.NoError(err)
		a.Equal(1, len(res.ModelDefList))
		a.Equal("sqlflow/my_test_model", res.ModelDefList[0].ImageUrl)
		a.Equal("DNNClassifier", res.ModelDefList[0].ClassName)
		a.Equal(307, len(res.ModelDefList[0].ArgDescs))
	})

	t.Run("ReleaseTrainedModel", func(t *testing.T) {
		stream, err := client.ReleaseModel(context.Background())
		a.NoError(err)
		// a random binary data to represent a trained model
		token := make([]byte, 256)
		rand.Read(token)
		req := &pb.ReleaseModelRequest{
			Name:              "my_regression_model",
			Tag:               "v0.1",
			Description:       "A linear regression model for house price predicting",
			EvaluationMetrics: "MSE: 0.02, MAPE: 10.32",
			ModelClassName:    "DNNClassifier",
			ModelRepoImageUrl: "sqlflow/my_test_model:v0.1",
			ContentTar:        token,
			ContentUrl:        "",
		}
		err = stream.Send(req)
		a.NoError(err)
		reply, err := stream.CloseAndRecv()
		a.NoError(err)
		a.Equal(true, reply.Success)

		listTrainedModelRes, err := client.ListModels(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
		a.NoError(err)
		a.Equal(1, len(listTrainedModelRes.ModelList))
		a.Equal("my_regression_model", listTrainedModelRes.ModelList[0].Name)
		a.Equal("sqlflow/my_test_model:v0.1", listTrainedModelRes.ModelList[0].ImageUrl)
	})

	t.Run("DropModels", func(t *testing.T) {
		_, err = client.DropModel(context.Background(), &pb.ReleaseModelRequest{
			Name: "my_regression_model", Tag: "v0.1",
		})

		listTrainedModelRes, err := client.ListModels(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
		a.NoError(err)
		a.Equal(0, len(listTrainedModelRes.ModelList))

		_, err = client.DropModelRepo(context.Background(),
			&pb.ReleaseModelRepoRequest{Name: "sqlflow/my_test_model", Tag: "v0.1"})
		a.NoError(err)

		res, err := client.ListModelRepos(context.Background(), &pb.ListModelRequest{Start: 0, Size: -1})
		a.NoError(err)
		a.Equal(0, len(res.ModelDefList))
	})
}
