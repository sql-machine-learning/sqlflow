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

package sql

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/test"
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
	testTrainSelectIris = `
SELECT * FROM iris.train
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
	testClusteringTrain = `SELECT (sepal_length - 4.4) / 3.5 as sepal_length, (sepal_width - 2.0) / 2.2 as sepal_width, (petal_length - 1) / 5.9 as petal_length, (petal_width - 0.1) / 2.4 as petal_width
FROM iris.train
TO TRAIN sqlflow_models.DeepEmbeddingClusterModel
WITH
  model.pretrain_dims = [10,10,3],
  model.n_clusters = 3,
  model.pretrain_epochs=5,
  train.batch_size=10,
  train.verbose=1
INTO sqlflow_models.my_clustering_model;
`
	testClusteringPredict = `
SELECT (sepal_length - 4.4) / 3.5 as sepal_length, (sepal_width - 2.0) / 2.2 as sepal_width, (petal_length - 1) / 5.9 as petal_length, (petal_width - 0.1) / 2.4 as petal_width
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
	testExplainTreeModelSelectIris = `
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
`, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecuteXGBoostClassifier(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectWithLimit, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
	a.NotPanics(func() {
		stream := RunSQLProgram(testXGBoostTrainSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testExplainTreeModelSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecuteXGBoostRegression(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testXGBoostTrainSelectHousing, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testExplainTreeModelSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testXGBoostPredictHousing, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNN(t *testing.T) {
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testPredictSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictClusteringLocalFS(t *testing.T) {
	t.Skip("fix random nan loss error then re-enable this test")
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := RunSQLProgram(testClusteringTrain, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testClusteringPredict, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictDNNLocalFS(t *testing.T) {
	a := assert.New(t)
	modelDir, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(modelDir)
	a.NotPanics(func() {
		stream := RunSQLProgram(testTrainSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
		stream = RunSQLProgram(testPredictSelectIris, modelDir, database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestExecutorTrainAndPredictionDNNClassifierDENSE(t *testing.T) {
	if test.GetEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", test.GetEnv("SQLFLOW_TEST_DB", "mysql")))
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
COLUMN DENSE(dense, 4, COMMA)
LABEL class
INTO sqlflow_models.my_dense_dnn_model;`
		stream := RunSQLProgram(trainSQL, "", database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))

		predSQL := `SELECT * FROM iris.test_dense
TO PREDICT iris.predict_dense.class
USING sqlflow_models.my_dense_dnn_model
;`
		stream = RunSQLProgram(predSQL, "", database.GetSessionFromTestingDB())
		a.True(test.GoodStream(stream.ReadAll()))
	})
}

func TestRewriteStatementsWithHints4Alisa(t *testing.T) {
	if os.Getenv("SQLFLOW_submitter") != "alisa" {
		t.Skip("Skip test case: submitter is not alisa.")
	}
	dialect := "alisa"
	hint1, hint2 := `set odps.stage.mapper.num=1;`, `set odps.sql.mapper.split.size=4096;`
	standardSQL, extendedSQL := `select 1;`, `select 1 to predict d.t.f using m;`
	sqlProgram := hint1 + "\n" + standardSQL + hint2 + extendedSQL

	a := assert.New(t)
	stmts, err := parser.Parse(dialect, sqlProgram)
	a.NoError(err)
	a.Equal(len(stmts), 4)

	sqls := RewriteStatementsWithHints(stmts, dialect)
	a.Equal(len(sqls), 2)
	a.Equal(sqls[0].Original, hint1+hint2+"\n"+standardSQL)
	a.Equal(sqls[1].Original, extendedSQL)
}

func TestIsHints(t *testing.T) {
	a := assert.New(t)
	a.True(isAlisaHint("set odps=2"))
	a.True(isAlisaHint(`--comment1
    --comment2
	set odps=2`))
	a.True(isAlisaHint(`--comment1
	set odps = 3
    --comment2`))
	a.True(isAlisaHint("-- comment \n set odps=2"))

	a.False(isAlisaHint("-- set odps=2"))
	a.False(isAlisaHint("-- comment \n -- set odps=2"))
}

func TestSQLLexerError(t *testing.T) {
	a := assert.New(t)
	stream := RunSQLProgram("SELECT * FROM ``?[] AS WHERE LIMIT;", "", database.GetSessionFromTestingDB())
	a.False(test.GoodStream(stream.ReadAll()))
}

func TestInitializeAndCheckAttributes(t *testing.T) {
	a := assert.New(t)
	wrong := "SELECT * FROM t1 TO TRAIN DNNClassifier WITH model.stddev=0.1 LABEL c INTO m;"
	r, e := parser.ParseStatement("mysql", wrong)
	a.NoError(e)
	trainStmt, err := ir.GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.Error(initializeAndCheckAttributes(trainStmt))

	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		model.optimizer="adam",
		model.hidden_units=[128,64]
	LABEL c4
	INTO mymodel;
	`
	r, e = parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err = ir.GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.NoError(initializeAndCheckAttributes(trainStmt))
	a.NoError(err)
	a.Equal("DNNClassifier", trainStmt.Estimator)
	a.Equal("SELECT c1, c2, c3, c4 FROM my_table\n	", trainStmt.Select)
	extendedAttr := map[string]bool{
		"train.epoch":                  true,
		"train.verbose":                true,
		"train.save_checkpoints_steps": true,
		"train.log_every_n_iter":       true,
		"train.max_steps":              true,
		"validation.steps":             true,
		"validation.metrics":           true,
		"validation.start_delay_secs":  true,
		"train.batch_size":             true,
		"validation.throttle_secs":     true,
		"validation.select":            true,
	}
	a.Equal(14, len(trainStmt.Attributes))

	for key, attr := range trainStmt.Attributes {
		if key == "model.n_classes" {
			a.Equal(2, attr.(int))
		} else if key == "model.optimizer" {
			a.Equal("adam()", attr.(string))
		} else if key == "model.hidden_units" {
			l, ok := attr.([]interface{})
			a.True(ok)
			a.Equal(128, l[0].(int))
			a.Equal(64, l[1].(int))
		} else if _, ok := extendedAttr[key]; !ok {
			a.Failf("error key", key)
		}
	}

	l, ok := trainStmt.Label.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c4", l.FieldDesc.Name)

	a.Equal("mymodel", trainStmt.Into)
}
