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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbeddingColumn(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	normal := statementWithColumn("EMBEDDING(CATEGORY_ID(c1, 100), 200, mean)")
	badInput := statementWithColumn("EMBEDDING(c1, 100, mean)")
	badBucket := statementWithColumn("EMBEDDING(CATEGORY_ID(c1, 100), bad, mean)")

	r, e := parser.Parse(normal)
	a.NoError(e)
	c := r.columns["feature_columns"]
	fcs, _, e := resolveTrainColumns(&c)
	a.NoError(e)
	ec, ok := fcs[0].(*embeddingColumn)
	a.True(ok)
	code, e := ec.GenerateCode()
	a.NoError(e)
	cc, ok := ec.CategoryColumn.(*categoryIDColumn)
	a.True(ok)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal(200, ec.Dimension)
	a.Equal("tf.feature_column.embedding_column(tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100), dimension=200, combiner=\"mean\")", code)

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
