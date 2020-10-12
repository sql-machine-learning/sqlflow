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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureColumnGenPythonCode(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skipf("skip TestFeatureColumnGenPythonCode for %s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	nc := &NumericColumn{
		FieldDesc: &FieldDesc{
			Name:  "testcol",
			Shape: []int{10},
			DType: 0,
		},
	}
	a.Equal(`runtime.feature.column.NumericColumn(runtime.feature.field_desc.FieldDesc(name="testcol", dtype=runtime.feature.field_desc.DataType.INT64, dtype_weight=runtime.feature.field_desc.DataType.INT64, delimiter="", delimiter_kv="", format="", shape=[10], is_sparse=False, vocabulary=[]))`,
		nc.GenPythonCode())

	emd := &EmbeddingColumn{
		CategoryColumn: nil,
		Dimension:      128,
	}
	a.Equal("runtime.feature.column.EmbeddingColumn(category_column=None, dimension=128, combiner=\"\", initializer=\"\", name=\"\")",
		emd.GenPythonCode())
}
