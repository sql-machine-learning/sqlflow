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

package sql

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

const (
	testTrainSelectWithLimit = `
SELECT * FROM iris.train
liMIT 10
TO TRAIN xgboost.gbtree
WITH
    objective="multi:softprob",
    train.num_boost_round = 30,
    eta = 0.4,
    num_class = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_xgboost_model;
`
	testTrainSelectIris = testSelectIris + `
TO TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
`
	testPredictSelectIris = `
SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;
`
	testClusteringTrain = `SELECT sepal_length, sepal_width, petal_length, petal_width
FROM iris.train
TO TRAIN sqlflow_models.DeepEmbeddingClusterModel
WITH
  model.pretrain_dims = [10,10],
  model.n_clusters = 3,
  model.pretrain_lr = 0.001,
  train.batch_size = 1
COLUMN sepal_length, sepal_width, petal_length, petal_width
INTO sqlflow_models.my_clustering_model;
`
	testClusteringPredict = `
SELECT sepal_length, sepal_width, petal_length, petal_width
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_clustering_model;
`
	testXGBoostTrainSelectIris = ` 
SELECT *
FROM iris.train
TO TRAIN xgboost.gbtree
WITH
    objective="multi:softprob",
    train.num_boost_round = 30,
    eta = 0.4,
    num_class = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_xgboost_model;
`
	testAnalyzeTreeModelSelectIris = `
SELECT * FROM iris.train
TO EXPLAIN sqlflow_models.my_xgboost_model
USING TreeExplainer;
`
	testXGBoostPredictIris = ` 
SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_xgboost_model;
`
	testXGBoostTrainSelectHousing = `
SELECT *
FROM housing.train
TO TRAIN xgboost.gbtree
WITH
	objective="reg:squarederror",
	train.num_boost_round = 30,
	validation.select="SELECT * FROM housing.train LIMIT 20"
COLUMN f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13
LABEL target
INTO sqlflow_models.my_xgb_regression_model;	
`
	testXGBoostPredictHousing = `
SELECT *
FROM housing.test
TO PREDICT housing.xgb_predict.target
USING sqlflow_models.my_xgb_regression_model;
`
)

func getDefaultSession() *pb.Session {
	return &pb.Session{
		DbConnStr: fmt.Sprintf("%s://%s", testDB.driverName, testDB.dataSourceName),
	}
}

func TestRunSQLProgram(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(`
SELECT sepal_length as sl, sepal_width as sw, class FROM iris.train
TO TRAIN xgboost.gbtree
WITH
    objective="multi:softprob",
    train.num_boost_round = 30,
    eta = 0.4,
    num_class = 3
LABEL class
INTO sqlflow_models.my_xgboost_model_by_program;

SELECT sepal_length as sl, sepal_width as sw FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_xgboost_model_by_program;

SELECT sepal_length as sl, sepal_width as sw, class FROM iris.train
TO EXPLAIN sqlflow_models.my_xgboost_model_by_program
USING TreeExplainer;
`, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})

}

func TestExecuteXGBoostClassifier(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectWithLimit, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := RunSQLProgram(testXGBoostTrainSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testAnalyzeTreeModelSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecuteXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testXGBoostTrainSelectHousing, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testAnalyzeTreeModelSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictHousing, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNN(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testPredictSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictClusteringLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := RunSQLProgram(testClusteringTrain, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testClusteringPredict, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNNLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = RunSQLProgram(testPredictSelectIris, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictionDNNClassifierDENSE(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", getEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)
	a.NotPanics(func() {
		trainSQL := `SELECT * FROM iris.train_dense
TO TRAIN DNNClassifier
WITH
model.n_classes = 3,
model.hidden_units = [10, 20],
train.epoch = 200,
train.batch_size = 10,
train.verbose = 1
COLUMN NUMERIC(dense, 4)
LABEL class
INTO sqlflow_models.my_dense_dnn_model;`
		stream := RunSQLProgram(trainSQL, "", getDefaultSession())
		a.True(goodStream(stream.ReadAll()))

		predSQL := `SELECT * FROM iris.test_dense
TO PREDICT iris.predict_dense.class
USING sqlflow_models.my_dense_dnn_model
;`
		stream = RunSQLProgram(predSQL, "", getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestCreatePredictionTable(t *testing.T) {
	a := assert.New(t)
	trainParsed, e := newExtendedSyntaxParser().Parse(testTrainSelectIris)
	a.NoError(e)
	predParsed, e := newExtendedSyntaxParser().Parse(testPredictSelectIris)
	a.NoError(e)
	predParsed.trainClause = trainParsed.trainClause
	a.NoError(createPredictionTable(predParsed, testDB, nil))
}

func TestLogChanWriter_Write(t *testing.T) {
	a := assert.New(t)
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		cw := &logChanWriter{wr: wr}
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("hello\n世界"))
		cw.Write([]byte("\n"))
		cw.Write([]byte("世界\n世界\n世界\n"))
	}()

	c := rd.ReadAll()

	a.Equal("hello\n", <-c)
	a.Equal("世界hello\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	a.Equal("世界\n", <-c)
	_, more := <-c
	a.False(more)
}

func TestSubmitWorkflow(t *testing.T) {
	if os.Getenv("SQLFLOW_ARGO_MODE") != "True" {
		t.Skip("argo: skip Argo tests")
	}
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		rd := SubmitWorkflow(testXGBoostTrainSelectIris, modelDir, getDefaultSession())
		for r := range rd.ReadAll() {
			switch r.(type) {
			case WorkflowJob:
				job := r.(WorkflowJob)
				a.True(strings.HasPrefix(job.JobID, "sqlflow-couler"))
				// TODO(tony): wait to check if job succeeded.
				// The workflow is currently failed since we haven't configure the data source.
			default:
				a.Fail("SubmitWorkflow should return JobID")
			}
		}
	})
}
