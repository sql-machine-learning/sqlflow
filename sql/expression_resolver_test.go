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
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/sql/columns"
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

func TestExecResource(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	s := statementWithAttrs("exec.worker_num = 2")
	r, e := parser.Parse(s)
	a.NoError(e)
	attrs, err := resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr := attrs["exec.worker_num"]
	a.Equal(attr.Value, "2")
}

func TestResolveAttrs(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	s := statementWithAttrs("estimator.hidden_units = [10, 20]")
	r, e := parser.Parse(s)
	a.NoError(e)
	attrs, err := resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr := attrs["estimator.hidden_units"]
	a.Equal("estimator", attr.Prefix)
	a.Equal("hidden_units", attr.Name)
	a.Equal([]interface{}([]interface{}{10, 20}), attr.Value)

	s = statementWithAttrs("dataset.name = hello")
	r, e = parser.Parse(s)
	a.NoError(e)
	attrs, err = resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr = attrs["dataset.name"]
	a.Equal("dataset", attr.Prefix)
	a.Equal("name", attr.Name)
	a.Equal("hello", attr.Value)

	s = statementWithAttrs("optimizer.learning_rate = 0.01")
	r, e = parser.Parse(s)
	a.NoError(e)
	attrs, err = resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr = attrs["optimizer.learning_rate"]
	a.Equal("optimizer", attr.Prefix)
	a.Equal("learning_rate", attr.Name)
	a.Equal("0.01", attr.Value)

	s = statementWithAttrs("model.n_classes = 2")
	r, e = parser.Parse(s)
	a.NoError(e)
	attrs, err = resolveAttribute(&r.trainAttrs)
	a.NoError(err)
	attr = attrs["model.n_classes"]
	a.Equal("model", attr.Prefix)
	a.Equal("n_classes", attr.Name)
	a.Equal("2", attr.Value)

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
	fcs, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	bc, ok := fcs[0].(*columns.BucketColumn)
	a.True(ok)
	code, e := bc.GenerateCode(nil)
	a.NoError(e)
	a.Equal("c1", bc.SourceColumn.Key)
	a.Equal([]int{10}, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=[10]), boundaries=[1,10])", code[0])

	r, e = parser.Parse(badInput)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBoundaries)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
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
	cc, ok := fcList[0].(*columns.CrossColumn)
	a.True(ok)
	code, e := cc.GenerateCode(nil)
	a.NoError(e)

	bc := cc.Keys[0].(*columns.BucketColumn)
	a.Equal("c1", bc.SourceColumn.Key)
	a.Equal([]int{10}, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("c5", cc.Keys[1].(string))
	a.Equal(20, cc.HashBucketSize)
	a.Equal("tf.feature_column.crossed_column([tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=[10]), boundaries=[1,10]),\"c5\"], hash_bucket_size=20)", code[0])

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

	normal := statementWithColumn("CATEGORY_ID(c1, 100)")
	badKey := statementWithColumn("CATEGORY_ID([100], 100)")
	badBucket := statementWithColumn("CATEGORY_ID(c1, bad)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcs, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	cc, ok := fcs[0].(*columns.CategoryIDColumn)
	a.True(ok)
	code, e := cc.GenerateCode(nil)
	a.NoError(e)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal("tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100)", code[0])

	r, e = parser.Parse(badKey)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBucket)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestCatIdColumnWithColumnSpec(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	dense := statementWithColumn("CATEGORY_ID(DENSE(col1, 128, COMMA), 100)")

	r, e := parser.Parse(dense)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcs, css, e := resolveTrainColumns(&c)
	a.NoError(e)
	_, ok := fcs[0].(*columns.CategoryIDColumn)
	a.True(ok)
	a.Equal(css[0].ColumnName, "col1")
}

func TestEmbeddingColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("EMBEDDING(CATEGORY_ID(c1, 100), 200, mean)")
	badInput := statementWithColumn("EMBEDDING(c1, 100)")
	badBucket := statementWithColumn("EMBEDDING(CATEGORY_ID(c1, 100), bad, mean)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcs, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	ec, ok := fcs[0].(*columns.EmbeddingColumn)
	a.True(ok)
	code, e := ec.GenerateCode(nil)
	a.NoError(e)
	cc, ok := ec.CategoryColumn.(*columns.CategoryIDColumn)
	a.True(ok)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal(200, ec.Dimension)
	a.Equal("tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100), dimension=200, combiner=\"mean\")", code[0])

	r, e = parser.Parse(badInput)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badBucket)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)
}

func TestNumericColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("NUMERIC(c2, [5, 10])")
	moreArgs := statementWithColumn("NUMERIC(c1, 100, args)")
	badShape := statementWithColumn("NUMERIC(c1, bad)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcs, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	nc, ok := fcs[0].(*columns.NumericColumn)
	a.True(ok)
	code, e := nc.GenerateCode(nil)
	a.NoError(e)
	a.Equal("c2", nc.Key)
	a.Equal([]int{5, 10}, nc.Shape)
	a.Equal("tf.feature_column.numeric_column(\"c2\", shape=[5,10])", code[0])

	r, e = parser.Parse(moreArgs)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)

	r, e = parser.Parse(badShape)
	a.NoError(e)
	c = r.columns["feature_columns"]
	fcs, _, e = resolveTrainColumns(&c)
	a.Error(e)
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
	_, css, e := resolveTrainColumns(&c)
	a.NoError(e)
	cs := css[0]
	a.Equal("c2", cs.ColumnName)
	a.Equal(5, cs.Shape[0])
	a.Equal(",", cs.Delimiter)
	a.Equal(false, cs.IsSparse)
	a.Equal("DenseColumn(name=\"c2\", shape=[5], dtype=\"float\", separator=\",\")", cs.ToString())

	r, e = parser.Parse(sparseStatement)
	a.NoError(e)
	c = r.columns["feature_columns"]
	_, css, e = resolveTrainColumns(&c)
	a.NoError(e)
	cs = css[0]
	a.Equal("c1", cs.ColumnName)
	a.Equal(100, cs.Shape[0])
	a.Equal(true, cs.IsSparse)
	a.Equal("SparseColumn(name=\"c1\", shape=[100], dtype=\"int\")", cs.ToString())

	r, e = parser.Parse(badStatement)
	a.NoError(e)
	c = r.columns["feature_columns"]
	_, _, e = resolveTrainColumns(&c)
	a.Error(e)

}
