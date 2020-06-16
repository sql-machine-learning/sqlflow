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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/modelzooserver"
)

const modelZooServerPort = 50055

func startTestModelZooServer() {
	addr := fmt.Sprintf("localhost:%d", modelZooServerPort)
	if serverIsReady(addr, time.Second) {
		return
	}
	go modelzooserver.StartModelZooServer(
		modelZooServerPort, database.GetTestingMySQLURL())
	waitForGivenServer(addr)
}

func prepareModelRepo() (string, error) {
	path, err := ioutil.TempDir("/tmp", "test_model_repo")
	if err != nil {
		return "", err
	}
	ioutil.WriteFile(fmt.Sprintf("%s/Dockerfile", path),
		[]byte(`FROM ubuntu:18.04
		COPY model /work/model`), 0644)
	os.Mkdir(fmt.Sprintf("%s/model", path), 0755)
	ioutil.WriteFile(fmt.Sprintf("%s/model/__init__.py", path),
		[]byte("from .my_model import MyDNNClassifier"), 0644)
	ioutil.WriteFile(fmt.Sprintf("%s/model/my_model.py", path), []byte(`
import tensorflow as tf

"""This is a test model"""
class MyDNNClassifier(tf.keras.Model):
	pass
`), 0644)
	return path, nil
}

func prepareModel() error {
	return runStmt(
		serverAddr,
		"SELECT * FROM iris.train WHERE class < 2 TO TRAIN LinearClassifier LABEL class INTO iris.my_model;",
		true,
		dbConnStr)
}

func caseReleaseRepo(t *testing.T) {
	a := assert.New(t)
	path, err := prepareModelRepo()
	a.NoError(err)
	defer os.RemoveAll(path)
	cmd := fmt.Sprintf(
		"--model-zoo-server=localhost:%d release repo %s test/my_repo v1.0",
		modelZooServerPort, path)
	opts, err := getOptions(cmd)
	a.NoError(err)
	a.NoError(releaseRepo(opts))
}

func caseDeleteRepo(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf("--model-zoo-server=localhost:%d delete repo test/my_repo v1.0",
		modelZooServerPort)
	opts, err := getOptions(cmd)
	a.NoError(err)
	a.NoError(deleteRepo(opts))
}

func caseReleaseModel(t *testing.T) {
	a := assert.New(t)
	a.NoError(prepareModel())
	cmd := fmt.Sprintf(
		`--model-zoo-server=localhost:%d --data-source=%s release model %s v1.0`,
		modelZooServerPort, database.GetTestingMySQLURL(), "iris.my_model")
	opts, err := getOptions(cmd)
	a.NoError(err)
	a.NoError(releaseModel(opts))
}

func CaseDeleteModel(t *testing.T) {
	a := assert.New(t)
	cmd := fmt.Sprintf(
		"--model-zoo-server=localhost:%d delete model iris.my_model v1.0",
		modelZooServerPort)
	opts, err := getOptions(cmd)
	a.NoError(err)
	a.NoError(deleteModel(opts))
}

func TestModelZooOperation(t *testing.T) {
	a := assert.New(t)
	startTestModelZooServer()
	stopServer := startServer()
	defer stopServer()
	waitForServer()
	a.NoError(prepareTestDataOrSkip(t))

	t.Run("caseReleaseRepo", caseReleaseRepo)
	t.Run("caseReleaseModel", caseReleaseModel)
	t.Run("caseDeleteModel", CaseDeleteModel)
	t.Run("caseDeleteRepo", caseDeleteRepo)
}
