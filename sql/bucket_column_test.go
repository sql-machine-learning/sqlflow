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
	bc, ok := fcs[0].(*bucketColumn)
	a.True(ok)
	code, e := bc.GenerateCode()
	a.NoError(e)
	a.Equal("c1", bc.SourceColumn.Key)
	a.Equal([]int{10}, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=[10]), boundaries=[1,10])", code)

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
