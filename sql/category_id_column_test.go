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
	cc, ok := fcs[0].(*categoryIDColumn)
	a.True(ok)
	code, e := cc.GenerateCode()
	a.NoError(e)
	a.Equal("c1", cc.Key)
	a.Equal(100, cc.BucketSize)
	a.Equal("tf.feature_column.categorical_column_with_identity(key=\"c1\", num_buckets=100)", code)

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
	_, ok := fcs[0].(*categoryIDColumn)
	a.True(ok)
	a.Equal(css[0].ColumnName, "col1")
}
