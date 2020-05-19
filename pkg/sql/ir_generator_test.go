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

	"sqlflow.org/sqlflow/pkg/database"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/parser"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func TestGenerateTrainStmt(t *testing.T) {
	a := assert.New(t)
	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		train.optimizer="adam",
		model.stddev=0.001,
		model.hidden_units=[128,64],
		validation.select="SELECT c1, c2, c3, c4 FROM my_table LIMIT 10"
	COLUMN c1,NUMERIC(c2, [128, 32]),CATEGORY_ID(c3, 512),
		SEQ_CATEGORY_ID(c3, 512),
		CROSS([c1,c2], 64),
		BUCKET(NUMERIC(c1, [100]), 100),
		EMBEDDING(CATEGORY_ID(c3, 512), 128, mean),
		NUMERIC(DENSE(c1, 64, COMMA), [128]),
		CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
		SEQ_CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
		EMBEDDING(c1, 128, sum),
		EMBEDDING(SPARSE(c2, 10000, COMMA, "int"), 128, sum),
		INDICATOR(CATEGORY_ID(c3, 512)),
		INDICATOR(c1),
		INDICATOR(SPARSE(c2, 10000, COMMA, "int"))
	LABEL c4
	INTO mymodel;
	`

	r, e := parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err := generateTrainStmt(r.SQLFlowSelectStmt, true)
	a.Error(err)
	trainStmt, err = generateTrainStmt(r.SQLFlowSelectStmt, false)
	a.NoError(err)
	a.Equal("DNNClassifier", trainStmt.Estimator)
	a.Equal(`SELECT c1, c2, c3, c4 FROM my_table
	`, trainStmt.Select)
	a.Equal("SELECT c1, c2, c3, c4 FROM my_table LIMIT 10", trainStmt.ValidationSelect)

	for key, attr := range trainStmt.Attributes {
		if key == "model.n_classes" {
			a.Equal(2, attr.(int))
		} else if key == "train.optimizer" {
			a.Equal("adam", attr.(string))
		} else if key == "model.stddev" {
			a.Equal(float32(0.001), attr.(float32))
		} else if key == "model.hidden_units" {
			l, ok := attr.([]interface{})
			a.True(ok)
			a.Equal(128, l[0].(int))
			a.Equal(64, l[1].(int))
		} else if key != "validation.select" {
			a.Failf("error key", key)
		}
	}

	nc, ok := trainStmt.Features["feature_columns"][0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal([]int{1}, nc.FieldDesc.Shape)

	nc, ok = trainStmt.Features["feature_columns"][1].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c2", nc.FieldDesc.Name)
	a.Equal([]int{128, 32}, nc.FieldDesc.Shape)

	cc, ok := trainStmt.Features["feature_columns"][2].(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cc.FieldDesc.Name)
	a.Equal(int64(512), cc.BucketSize)

	seqcc, ok := trainStmt.Features["feature_columns"][3].(*ir.SeqCategoryIDColumn)
	a.True(ok)
	a.Equal("c3", seqcc.FieldDesc.Name)

	cross, ok := trainStmt.Features["feature_columns"][4].(*ir.CrossColumn)
	a.True(ok)
	a.Equal("c1", cross.Keys[0].(string))
	a.Equal("c2", cross.Keys[1].(string))
	a.Equal(64, cross.HashBucketSize)

	bucket, ok := trainStmt.Features["feature_columns"][5].(*ir.BucketColumn)
	a.True(ok)
	a.Equal(100, bucket.Boundaries[0])
	a.Equal("c1", bucket.SourceColumn.FieldDesc.Name)

	emb, ok := trainStmt.Features["feature_columns"][6].(*ir.EmbeddingColumn)
	a.True(ok)
	a.Equal("mean", emb.Combiner)
	a.Equal(128, emb.Dimension)
	embInner, ok := emb.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", embInner.FieldDesc.Name)
	a.Equal(int64(512), embInner.BucketSize)

	// NUMERIC(DENSE(c1, [64], COMMA), [128])
	nc, ok = trainStmt.Features["feature_columns"][7].(*ir.NumericColumn)
	a.True(ok)
	a.Equal(64, nc.FieldDesc.Shape[0])
	a.Equal(",", nc.FieldDesc.Delimiter)
	a.False(nc.FieldDesc.IsSparse)

	// CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
	cc, ok = trainStmt.Features["feature_columns"][8].(*ir.CategoryIDColumn)
	a.True(ok)
	a.True(cc.FieldDesc.IsSparse)
	a.Equal("c2", cc.FieldDesc.Name)
	a.Equal(10000, cc.FieldDesc.Shape[0])
	a.Equal(",", cc.FieldDesc.Delimiter)
	a.Equal(int64(128), cc.BucketSize)

	// SEQ_CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128)
	scc, ok := trainStmt.Features["feature_columns"][9].(*ir.SeqCategoryIDColumn)
	a.True(ok)
	a.True(scc.FieldDesc.IsSparse)
	a.Equal("c2", scc.FieldDesc.Name)
	a.Equal(10000, scc.FieldDesc.Shape[0])

	// EMBEDDING(c1, 128)
	emb, ok = trainStmt.Features["feature_columns"][10].(*ir.EmbeddingColumn)
	a.True(ok)
	a.Equal(nil, emb.CategoryColumn)
	a.Equal(128, emb.Dimension)

	// EMBEDDING(SPARSE(c2, 10000, COMMA, "int"), 128)
	emb, ok = trainStmt.Features["feature_columns"][11].(*ir.EmbeddingColumn)
	a.True(ok)
	catCol, ok := emb.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])
	a.Equal(",", catCol.FieldDesc.Delimiter)

	// INDICATOR(CATEGORY_ID(c3, 512)),
	ic, ok := trainStmt.Features["feature_columns"][12].(*ir.IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", catCol.FieldDesc.Name)
	a.Equal(int64(512), catCol.BucketSize)

	// INDICATOR(c1)
	ic, ok = trainStmt.Features["feature_columns"][13].(*ir.IndicatorColumn)
	a.True(ok)
	a.Equal(nil, ic.CategoryColumn)
	a.Equal("c1", ic.Name)

	// INDICATOR(SPARSE(c2, 10000, COMMA, "int"))
	ic, ok = trainStmt.Features["feature_columns"][14].(*ir.IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*ir.CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])

	l, ok := trainStmt.Label.(*ir.NumericColumn)
	a.True(ok)
	a.Equal("c4", l.FieldDesc.Name)

	a.Equal("mymodel", trainStmt.Into)
}

func TestGenerateTrainStmtWithTypeCheck(t *testing.T) {
	a := assert.New(t)
	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		model.optimizer="adam",
		model.hidden_units=[128,64]
	LABEL c4
	INTO mymodel;
	`

	r, e := parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err := generateTrainStmt(r.SQLFlowSelectStmt, true)
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

func TestGenerateTrainStmtModelZoo(t *testing.T) {
	a := assert.New(t)

	normal := `
	SELECT c1, c2, c3, c4
	FROM my_table
	TO TRAIN a_data_scientist/regressors:v0.2/MyDNNRegressor
	WITH
		model.n_classes=2,
		train.optimizer="adam"
	LABEL c4
	INTO mymodel;
	`

	r, e := parser.ParseStatement("mysql", normal)
	a.NoError(e)

	trainStmt, err := generateTrainStmt(r.SQLFlowSelectStmt, false)
	a.NoError(err)
	a.Equal("a_data_scientist/regressors:v0.2", trainStmt.ModelImage)
	a.Equal("MyDNNRegressor", trainStmt.Estimator)
}

func TestGeneratePredictStmt(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") == "hive" {
		t.Skip(fmt.Sprintf("%s: skip Hive test", getEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)

	predSQL := `SELECT * FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.mymodel;`
	r, e := parser.ParseStatement("mysql", predSQL)
	a.NoError(e)

	// need to save a model first because predict SQL will read the train SQL
	// from saved model
	cwd, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(cwd)
	modelDir := ""
	stream := RunSQLProgram(`SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[10,20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.mymodel;`, modelDir, &pb.Session{DbConnStr: database.GetTestingDBSingleton().URL()})
	a.True(goodStream(stream.ReadAll()))

	predStmt, err := generatePredictStmt(r.SQLFlowSelectStmt, database.GetTestingDBSingleton().URL(), modelDir, cwd, true)
	a.NoError(err)

	a.Equal("iris.predict", predStmt.ResultTable)
	a.Equal("class", predStmt.TrainStmt.Label.GetFieldDesc()[0].Name)
	a.Equal("DNNClassifier", predStmt.TrainStmt.Estimator)
	nc, ok := predStmt.TrainStmt.Features["feature_columns"][0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("sepal_length", nc.FieldDesc.Name)
	a.Equal("sqlflow_models.mymodel", predStmt.Using)
}

func TestGenerateExplainStmt(t *testing.T) {
	if getEnv("SQLFLOW_TEST_DB", "mysql") != "mysql" {
		t.Skip(fmt.Sprintf("%s: skip test", getEnv("SQLFLOW_TEST_DB", "mysql")))
	}
	a := assert.New(t)
	connStr := "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"

	cwd, e := ioutil.TempDir("/tmp", "sqlflow_models")
	a.Nil(e)
	defer os.RemoveAll(cwd)
	modelDir := ""
	stream := RunSQLProgram(`SELECT * FROM iris.train
TO TRAIN xgboost.gbtree
WITH
	objective="multi:softprob",
	train.num_boost_round = 30,
	eta = 0.4,
	num_class = 3
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_xgboost_model;
`, modelDir, &pb.Session{DbConnStr: connStr})
	a.NoError(e)
	a.True(goodStream(stream.ReadAll()))

	pr, e := parser.ParseStatement("mysql", `
	SELECT *
	FROM iris.train
	TO EXPLAIN sqlflow_models.my_xgboost_model
	WITH
	    summary.plot_type="bar",
	    summary.alpha=1,
	    summary.sort=True
	USING TreeExplainer;
	`)
	a.NoError(e)

	ExplainStmt, e := generateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainStmt.Attributes), 3)
	a.Equal(ExplainStmt.Attributes["summary.sort"], true)
	a.Equal(ExplainStmt.Attributes["summary.plot_type"], "bar")
	a.Equal(ExplainStmt.Attributes["summary.alpha"], 1)

	nc, ok := ExplainStmt.TrainStmt.Features["feature_columns"][0].(*ir.NumericColumn)
	a.True(ok)
	a.Equal("sepal_length", nc.FieldDesc.Name)

	pr, e = parser.ParseStatement("mysql", `
	SELECT *
	FROM iris.train
	TO EXPLAIN sqlflow_models.my_xgboost_model
	WITH
	    summary.plot_type="bar",
	    summary.alpha=1,
	    summary.sort=True
	USING TreeExplainer
	INTO db.explain_result;
	`)
	a.NoError(e)

	ExplainIntoStmt, e := generateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(ExplainIntoStmt.Explainer, "TreeExplainer")
	a.Equal(len(ExplainIntoStmt.Attributes), 3)
	a.Equal("db.explain_result", ExplainIntoStmt.Into)

	pr, e = parser.ParseStatement("mysql", `SELECT * FROM iris.train TO EXPLAIN sqlflow_models.my_xgboost_model;`)
	a.NoError(e)
	shortExplainStmt, e := generateExplainStmt(pr.SQLFlowSelectStmt, connStr, modelDir, cwd, true)
	a.NoError(e)
	a.Equal(shortExplainStmt.Explainer, "")
	a.Equal(len(shortExplainStmt.Attributes), 0)
}

func TestInferStringValue(t *testing.T) {
	a := assert.New(t)
	for _, s := range []string{"true", "TRUE", "True"} {
		a.Equal(inferStringValue(s), true)
		a.Equal(inferStringValue(fmt.Sprintf("\"%s\"", s)), s)
		a.Equal(inferStringValue(fmt.Sprintf("'%s'", s)), s)
	}
	for _, s := range []string{"false", "FALSE", "False"} {
		a.Equal(inferStringValue(s), false)
		a.Equal(inferStringValue(fmt.Sprintf("\"%s\"", s)), s)
		a.Equal(inferStringValue(fmt.Sprintf("'%s'", s)), s)
	}
	a.Equal(inferStringValue("t"), "t")
	a.Equal(inferStringValue("F"), "F")
	a.Equal(inferStringValue("1"), 1)
	a.Equal(inferStringValue("\"1\""), "1")
	a.Equal(inferStringValue("'1'"), "1")
	a.Equal(inferStringValue("2.3"), float32(2.3))
	a.Equal(inferStringValue("\"2.3\""), "2.3")
	a.Equal(inferStringValue("'2.3'"), "2.3")
}

func bucketColumnParserTestMain(bucketStr string) error {
	stmtStr := fmt.Sprintf(`
	SELECT petal_length, class
	FROM iris.train
	TO TRAIN sqlflow_models.my_bucket_column_model
	WITH model.batch_size = 32
	COLUMN BUCKET(%s)
	LABEL class
	INTO db.explain_result;
	`, bucketStr)

	pr, err := parser.Parse("mysql", stmtStr)

	if err != nil {
		return err
	}

	trainStmt, err := generateTrainStmt(pr[0].SQLFlowSelectStmt, false)
	if err != nil {
		return err
	}

	if _, ok := trainStmt.Features["feature_columns"][0].(*ir.BucketColumn); !ok {
		return fmt.Errorf("feature column should be BucketColumn")
	}

	return nil
}

func TestBucketColumnParser(t *testing.T) {
	a := assert.New(t)
	a.NoError(bucketColumnParserTestMain("NUMERIC(petal_length, 1), [0, 10]"))
	a.NoError(bucketColumnParserTestMain("NUMERIC(petal_length, 1), [-10, -5, 10]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [10, 20]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [-100]"))
	a.NoError(bucketColumnParserTestMain("petal_length, [-100, -50]"))

	a.Error(bucketColumnParserTestMain("NUMERIC(petal_length, 1), [10, 0]"))
	a.Error(bucketColumnParserTestMain("NUMERIC(petal_length, 1), [-10, -10]"))
	a.Error(bucketColumnParserTestMain("NUMERIC(petal_length, 1), [5, 5]"))
}
