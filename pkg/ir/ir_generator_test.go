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

package ir

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/parser"
)

func TestGenerateTrainStmt(t *testing.T) {
	a := assert.New(t)
	normal := `SELECT c1, c2, c3, c4 FROM my_table
	TO TRAIN DNNClassifier
	WITH
		model.n_classes=2,
		train.optimizer="adam",
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

	trainStmt, err := GenerateTrainStmt(r.SQLFlowSelectStmt)
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

	nc, ok := trainStmt.Features["feature_columns"][0].(*NumericColumn)
	a.True(ok)
	a.Equal([]int{1}, nc.FieldDesc.Shape)

	nc, ok = trainStmt.Features["feature_columns"][1].(*NumericColumn)
	a.True(ok)
	a.Equal("c2", nc.FieldDesc.Name)
	a.Equal([]int{128, 32}, nc.FieldDesc.Shape)

	cc, ok := trainStmt.Features["feature_columns"][2].(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", cc.FieldDesc.Name)
	a.Equal(int64(512), cc.BucketSize)

	seqcc, ok := trainStmt.Features["feature_columns"][3].(*SeqCategoryIDColumn)
	a.True(ok)
	a.Equal("c3", seqcc.FieldDesc.Name)

	cross, ok := trainStmt.Features["feature_columns"][4].(*CrossColumn)
	a.True(ok)
	a.Equal("c1", cross.Keys[0].(string))
	a.Equal("c2", cross.Keys[1].(string))
	a.Equal(64, cross.HashBucketSize)

	bucket, ok := trainStmt.Features["feature_columns"][5].(*BucketColumn)
	a.True(ok)
	a.Equal(100, bucket.Boundaries[0])
	a.Equal("c1", bucket.SourceColumn.FieldDesc.Name)

	emb, ok := trainStmt.Features["feature_columns"][6].(*EmbeddingColumn)
	a.True(ok)
	a.Equal("mean", emb.Combiner)
	a.Equal(128, emb.Dimension)
	embInner, ok := emb.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", embInner.FieldDesc.Name)
	a.Equal(int64(512), embInner.BucketSize)

	// NUMERIC(DENSE(c1, [64], COMMA), [128])
	nc, ok = trainStmt.Features["feature_columns"][7].(*NumericColumn)
	a.True(ok)
	a.Equal(64, nc.FieldDesc.Shape[0])
	a.Equal(",", nc.FieldDesc.Delimiter)
	a.False(nc.FieldDesc.IsSparse)

	// CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128),
	cc, ok = trainStmt.Features["feature_columns"][8].(*CategoryIDColumn)
	a.True(ok)
	a.True(cc.FieldDesc.IsSparse)
	a.Equal("c2", cc.FieldDesc.Name)
	a.Equal(10000, cc.FieldDesc.Shape[0])
	a.Equal(",", cc.FieldDesc.Delimiter)
	a.Equal(int64(128), cc.BucketSize)

	// SEQ_CATEGORY_ID(SPARSE(c2, 10000, COMMA), 128)
	scc, ok := trainStmt.Features["feature_columns"][9].(*SeqCategoryIDColumn)
	a.True(ok)
	a.True(scc.FieldDesc.IsSparse)
	a.Equal("c2", scc.FieldDesc.Name)
	a.Equal(10000, scc.FieldDesc.Shape[0])

	// EMBEDDING(c1, 128)
	emb, ok = trainStmt.Features["feature_columns"][10].(*EmbeddingColumn)
	a.True(ok)
	a.Equal(nil, emb.CategoryColumn)
	a.Equal(128, emb.Dimension)

	// EMBEDDING(SPARSE(c2, 10000, COMMA, "int"), 128)
	emb, ok = trainStmt.Features["feature_columns"][11].(*EmbeddingColumn)
	a.True(ok)
	catCol, ok := emb.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])
	a.Equal(",", catCol.FieldDesc.Delimiter)

	// INDICATOR(CATEGORY_ID(c3, 512)),
	ic, ok := trainStmt.Features["feature_columns"][12].(*IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.Equal("c3", catCol.FieldDesc.Name)
	a.Equal(int64(512), catCol.BucketSize)

	// INDICATOR(c1)
	ic, ok = trainStmt.Features["feature_columns"][13].(*IndicatorColumn)
	a.True(ok)
	a.Equal(nil, ic.CategoryColumn)
	a.Equal("c1", ic.Name)

	// INDICATOR(SPARSE(c2, 10000, COMMA, "int"))
	ic, ok = trainStmt.Features["feature_columns"][14].(*IndicatorColumn)
	a.True(ok)
	catCol, ok = ic.CategoryColumn.(*CategoryIDColumn)
	a.True(ok)
	a.True(catCol.FieldDesc.IsSparse)
	a.Equal("c2", catCol.FieldDesc.Name)
	a.Equal(10000, catCol.FieldDesc.Shape[0])

	l, ok := trainStmt.Label.(*NumericColumn)
	a.True(ok)
	a.Equal("c4", l.FieldDesc.Name)

	a.Equal("mymodel", trainStmt.Into)
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

	trainStmt, err := GenerateTrainStmt(pr[0].SQLFlowSelectStmt)
	if err != nil {
		return err
	}

	if _, ok := trainStmt.Features["feature_columns"][0].(*BucketColumn); !ok {
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

	trainStmt, err := GenerateTrainStmt(r.SQLFlowSelectStmt)
	a.NoError(err)
	a.Equal("a_data_scientist/regressors:v0.2", trainStmt.ModelImage)
	a.Equal("MyDNNRegressor", trainStmt.Estimator)
}
