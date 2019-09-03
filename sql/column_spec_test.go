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
