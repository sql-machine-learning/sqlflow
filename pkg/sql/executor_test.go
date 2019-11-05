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
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

const (
	testStandardExecutiveSQLStatement = `DELETE FROM iris.train WHERE class = 4;`
	testSelectIris                    = `
SELECT *
FROM iris.train
`
	testTrainSelectIris = testSelectIris + `
TRAIN DNNClassifier
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
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
`
	testClusteringTrain = `SELECT sepal_length, sepal_width, petal_length, petal_width
FROM iris.train
TRAIN sqlflow_models.DeepEmbeddingClusterModel
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
PREDICT iris.predict.class
USING sqlflow_models.my_clustering_model;
`
	testXGBoostTrainSelectIris = ` 
SELECT *
FROM iris.train
TRAIN xgboost.gbtree
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
ANALYZE sqlflow_models.my_xgboost_model
USING TreeExplainer;
`
	testXGBoostPredictIris = ` 
SELECT *
FROM iris.test
PREDICT iris.predict.class
USING sqlflow_models.my_xgboost_model;
`
)

func goodStream(stream chan interface{}) (bool, string) {
	lastResp := list.New()
	keepSize := 10

	for rsp := range stream {
		switch rsp.(type) {
		case error:
			var s []string
			for e := lastResp.Front(); e != nil; e = e.Next() {
				s = append(s, e.Value.(string))
			}
			return false, strings.Join(s, "\n")
		}
		lastResp.PushBack(rsp)
		if lastResp.Len() > keepSize {
			e := lastResp.Front()
			lastResp.Remove(e)
		}
	}
	return true, ""
}

func TestSplitExtendedSQL(t *testing.T) {
	a := assert.New(t)
	s, err := splitExtendedSQL(`select a train b with c;`)
	a.Equal(err, nil)
	a.Equal(2, len(s))
	a.Equal(`select a`, s[0])
	a.Equal(` train b with c;`, s[1])

	s, err = splitExtendedSQL(`  select a predict b using c;`)
	a.Equal(err, nil)
	a.Equal(2, len(s))
	a.Equal(`  select a`, s[0])
	a.Equal(` predict b using c;`, s[1])

	s, err = splitExtendedSQL(` select a from b;`)
	a.Equal(err, nil)
	a.Equal(1, len(s))
	a.Equal(` select a from b;`, s[0])

	s, err = splitExtendedSQL(`train a with b;`)
	a.Equal(err, nil)
	a.Equal(1, len(s))
	a.Equal(`train a with b;`, s[0])
}

func TestSplitMulipleSQL(t *testing.T) {
	a := assert.New(t)
	splited, err := SplitMultipleSQL(`CREATE TABLE copy_table_1 AS SELECT a,b,c FROM table_1 WHERE c<>";";
SELECT * FROM copy_table_1;SELECT * FROM copy_table_1 TO TRAIN DNNClassifier WITH n_classes=2 INTO test_model;`)
	a.NoError(err)
	a.Equal("CREATE TABLE copy_table_1 AS SELECT a,b,c FROM table_1 WHERE c<>\";\";", splited[0])
	a.Equal("SELECT * FROM copy_table_1;", splited[1])
	a.Equal("SELECT * FROM copy_table_1 TO TRAIN DNNClassifier WITH n_classes=2 INTO test_model;", splited[2])
}

func getDefaultSession() *pb.Session {
	return &pb.Session{}
}
func TestExecuteXGBoost(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := runExtendedSQL(testXGBoostTrainSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testAnalyzeTreeModelSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testXGBoostPredictIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecuteXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := runExtendedSQL(testXGBoostTrainSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testAnalyzeTreeModelSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testXGBoostPredictIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNN(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := runExtendedSQL(testTrainSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testPredictSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictClusteringLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := runExtendedSQL(testClusteringTrain, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = runExtendedSQL(testClusteringPredict, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNNLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := runExtendedSQL(testTrainSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))

		stream = runExtendedSQL(testPredictSelectIris, testDB, modelDir, getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictionDNNClassifierDENSE(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", getEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)
	a.NotPanics(func() {
		stream := Run(`SELECT * FROM iris.train_dense
TO TRAIN DNNClassifier
WITH
model.n_classes = 3,
model.hidden_units = [10, 20],
train.epoch = 200,
train.batch_size = 10,
train.verbose = 1
COLUMN NUMERIC(dense, 4)
LABEL class
INTO sqlflow_models.my_dense_dnn_model;
`, testDB, "", getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
		stream = Run(`SELECT * FROM iris.test_dense
TO PREDICT iris.predict_dense.class
USING sqlflow_models.my_dense_dnn_model
;`, testDB, "", getDefaultSession())
		a.True(goodStream(stream.ReadAll()))
	})
}

func TestStandardSQL(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		stream := runStandardSQL(testSelectIris, testDB)
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
			t.Skip("hive: skip DELETE statement")
		}
		stream := runStandardSQL(testStandardExecutiveSQLStatement, testDB)
		a.True(goodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := runStandardSQL("SELECT * FROM iris.iris_empty LIMIT 10;", testDB)
		stat, _ := goodStream(stream.ReadAll())
		a.True(stat)
	})
}

func TestSQLLexerError(t *testing.T) {
	a := assert.New(t)
	stream := Run("SELECT * FROM ``?[] AS WHERE LIMIT;", testDB, "", getDefaultSession())
	a.False(goodStream(stream.ReadAll()))
}

func TestCreatePredictionTable(t *testing.T) {
	a := assert.New(t)
	trainParsed, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)
	predParsed, e := newParser().Parse(testPredictSelectIris)
	a.NoError(e)
	predParsed.trainClause = trainParsed.trainClause
	a.NoError(createPredictionTable(predParsed, testDB, nil))
}

func TestIsQuery(t *testing.T) {
	a := assert.New(t)
	a.True(isQuery("select * from iris.iris"))
	a.True(isQuery("show create table iris.iris"))
	a.True(isQuery("show databases"))
	a.True(isQuery("show tables"))
	a.True(isQuery("describe iris.iris"))

	a.False(isQuery("select * from iris.iris limit 10 into iris.tmp"))
	a.False(isQuery("insert into iris.iris values ..."))
	a.False(isQuery("delete from iris.iris where ..."))
	a.False(isQuery("update iris.iris where ..."))
	a.False(isQuery("drop table"))
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

func TestParseTableColumn(tg *testing.T) {
	a := assert.New(tg)
	t, c, e := parseTableColumn("a.b.c")
	a.NoError(e)
	a.Equal("a.b", t)
	a.Equal("c", c)

	t, c, e = parseTableColumn("a.b")
	a.NoError(e)
	a.Equal("a", t)
	a.Equal("b", c)

	_, _, e = parseTableColumn("a.")
	a.Error(e)
	_, _, e = parseTableColumn("a")
	a.Error(e)
}
