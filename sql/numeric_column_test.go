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
	nc, ok := fcs[0].(*numericColumn)
	a.True(ok)
	code, e := nc.GenerateCode()
	a.NoError(e)
	a.Equal("c2", nc.Key)
	a.Equal([]int{5, 10}, nc.Shape)
	a.Equal("tf.feature_column.numeric_column(\"c2\", shape=[5,10])", code)

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
