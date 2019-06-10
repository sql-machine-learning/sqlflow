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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	trainStatement = `select c1, c2, c3 from kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH 
			%v
		COLUMN
			%v
		LABEL c3 INTO model_table;`
)

func statementWithColumn(column string) string {
	return fmt.Sprintf(trainStatement, "estimator.hidden_units = [10, 20]", column)
}

func statementWithAttrs(attrs string) string {
	return fmt.Sprintf(trainStatement, attrs, "DENSE(c2, 5, comma)")
}

func TestFeatureSpec(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	denseStatement := statementWithColumn("DENSE(c2, 5, comma)")
	sparseStatement := statementWithColumn("SPARSE(c1, 100, comma)")
	badStatement := statementWithColumn("DENSE(c3, bad, comma)")

	r, e := parser.Parse(denseStatement)
	a.NoError(e)
	c := r.columns["feature_columns"]
	_, fsMap, e := resolveTrainColumns(&c)
	a.NoError(e)
	fs := fsMap["c2"]
	a.Equal("c2", fs.FeatureName)
	a.Equal(5, fs.Shape[0])
	a.Equal(",", fs.Delimiter)
	a.Equal(false, fs.IsSparse)
	a.Equal("DenseColumn(name=\"c2\", shape=[5], dtype=\"float\", separator=\",\")", fs.ToString())

	r, e = parser.Parse(sparseStatement)
	a.NoError(e)
	c = r.columns["feature_columns"]
	_, fsMap, e = resolveTrainColumns(&c)
	a.NoError(e)
	fs = fsMap["c1"]
	a.Equal("c1", fs.FeatureName)
	a.Equal(100, fs.Shape[0])
	a.Equal(",", fs.Delimiter)
	a.Equal(true, fs.IsSparse)
	a.Equal("SparseColumn(name=\"c1\", shape=[100], dtype=\"float\", separator=\",\")", fs.ToString())

	r, e = parser.Parse(badStatement)
	a.NoError(e)
	c = r.columns["feature_columns"]
	_, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestNumericColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("NUMERIC(c2, 5)")
	moreArgs := statementWithColumn("NUMERIC(c1, 100, args)")
	badShape := statementWithColumn("NUMERIC(c1, bad)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcList, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	nc, ok := fcList[0].(*numericColumn)
	a.True(ok)
	code, e := nc.GenerateCode()
	a.NoError(e)
	a.Equal("c2", nc.Key)
	a.Equal(5, nc.Shape)
	a.Equal("tf.feature_column.numeric_column(\"c2\", shape=(5,))", code)

	r, e = parser.Parse(moreArgs)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badShape)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestBucketColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("BUCKET(NUMERIC(c1, 10), [1, 10])")
	badInput := statementWithColumn("BUCKET(c1, [1, 10])")
	badBoundaries := statementWithColumn("BUCKET(NUMERIC(c1, 10), 100)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcList, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	bc, ok := fcList[0].(*bucketColumn)
	a.True(ok)
	code, e := bc.GenerateCode()
	a.NoError(e)
	a.Equal("c1", bc.SourceColumn.Key)
	a.Equal(10, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=(10,)), boundaries=[1,10])", code)

	r, e = parser.Parse(badInput)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBoundaries)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestCrossColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("cross([BUCKET(NUMERIC(c1, 10), [1, 10]), c5], 20)")
	badInput := statementWithColumn("cross(c1, 20)")
	badBucketSize := statementWithColumn("cross([BUCKET(NUMERIC(c1, 10), [1, 10]), c5], bad)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcList, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	cc, ok := fcList[0].(*crossColumn)
	a.True(ok)
	code, e := cc.GenerateCode()
	a.NoError(e)

	bc := cc.Keys[0].(*bucketColumn)
	a.Equal("c1", bc.SourceColumn.Key)
	a.Equal(10, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("c5", cc.Keys[1].(string))
	a.Equal(20, cc.HashBucketSize)
	a.Equal("tf.feature_column.crossed_column([tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=(10,)), boundaries=[1,10]),\"c5\"], hash_bucket_size=20)", code)

	r, e = parser.Parse(badInput)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBucketSize)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestCatIdColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("CAT_ID(c1, 100)")
	badKey := statementWithColumn("CAT_ID([100], 100)")
	badBucket := statementWithColumn("CAT_ID(c1, bad)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcList, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	cc, ok := fcList[0].(*catIDColumn)
	a.True(ok)
	code, e := cc.GenerateCode()
	a.NoError(e)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal("tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100)", code)

	r, e = parser.Parse(badKey)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBucket)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestEmbeddingColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("EMBEDDING(CAT_ID(c1, 100), 200, mean)")
	badInput := statementWithColumn("EMBEDDING(c1, 100, mean)")
	badBucket := statementWithColumn("EMBEDDING(CAT_ID(c1, 100), bad, mean)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcList, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	ec, ok := fcList[0].(*embeddingColumn)
	a.True(ok)
	code, e := ec.GenerateCode()
	a.NoError(e)
	cc, ok := ec.CatColumn.(*catIDColumn)
	a.True(ok)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal(200, ec.Dimension)
	a.Equal("tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100), dimension=200, combiner=\"mean\")", code)

	r, e = parser.Parse(badInput)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBucket)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcList, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestAttrs(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	s := statementWithAttrs("estimator.hidden_units = [10, 20], dataset.name = hello")
	r, e := parser.Parse(s)
	a.NoError(e)
	attrs, err := resolveTrainAttribute(&r.attrs)
	a.NoError(err)
	a.Equal(2, len(attrs))
	a.Equal("estimator", attrs[0].Prefix)
	a.Equal("hidden_units", attrs[0].Name)
	a.Equal([]interface{}([]interface{}{10, 20}), attrs[0].Value)
	a.Equal("dataset", attrs[1].Prefix)
	a.Equal("name", attrs[1].Name)
	a.Equal("hello", attrs[1].Value)
}

func TestTrainALPSFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `select dense, deep, wide from kaggle_credit_fraud_training_data 
		TRAIN DNNLinearCombinedClassifier 
		WITH 
			estimator.dnn_hidden_units = [10, 20],
			train_spec.max_steps = 1000
		COLUMN
			DENSE(dense, 5, comma),
			SPARSE(deep, 2000, comma),
			NUMERIC(dense, 5),
			EMBEDDING(CAT_ID(deep, 2000), 8, mean) FOR dnn_feature_columns
		COLUMN
			SPARSE(wide, 1000, comma),
			EMBEDDING(CAT_ID(wide, 1000), 16, mean) FOR linear_feature_columns
		LABEL c3 INTO model_table;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)

	filler, e := newALPSTrainFiller(r)
	a.NoError(e)

	a.True(filler.IsTraining)
	a.Equal("kaggle_credit_fraud_training_data", filler.TrainInputTable)
	a.Equal("[\"dense\",\"deep\",\"wide\"]", filler.Fields)
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"deep\", shape=[2000], dtype=\"float\", separator=\",\")"))
	a.True(strings.Contains(filler.X, "SparseColumn(name=\"wide\", shape=[1000], dtype=\"float\", separator=\",\")"))
	a.True(strings.Contains(filler.X, "DenseColumn(name=\"dense\", shape=[5], dtype=\"float\", separator=\",\")"))
	a.Equal("DenseColumn(name=\"c3\", shape=[1], dtype=\"int\", separator=\",\")", filler.Y)
	a.Equal("tf.estimator.DNNLinearCombinedClassifier(dnn_hidden_units=[10,20],config=run_config,dnn_feature_columns=[tf.feature_column.numeric_column(\"dense\", shape=(5,)),tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"deep\", num_buckets=2000), dimension=8, combiner=\"mean\")],linear_feature_columns=[tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"wide\", num_buckets=1000), dimension=16, combiner=\"mean\")])", filler.EstimatorCreatorCode)
	a.Equal("1000", filler.TrainSpec["max_steps"])
}
