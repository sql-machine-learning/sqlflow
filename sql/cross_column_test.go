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
	a.Equal([]int{10}, bc.SourceColumn.Shape)
	a.Equal([]int{1, 10}, bc.Boundaries)
	a.Equal("c5", cc.Keys[1].(string))
	a.Equal(20, cc.HashBucketSize)
	a.Equal("tf.feature_column.crossed_column([tf.feature_column.bucketized_column(tf.feature_column.numeric_column(\"c1\", shape=[10]), boundaries=[1,10]),\"c5\"], hash_bucket_size=20)", code)

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
