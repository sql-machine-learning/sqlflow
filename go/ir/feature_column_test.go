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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureColumnGenPythonCode(t *testing.T) {
	a := assert.New(t)
	nc := &NumericColumn{
		FieldDesc: &FieldDesc{
			Name:  "testcol",
			Shape: []int{10},
			DType: 0,
		},
	}
	a.Equal("runtime.feature.column.NumericColumn(runtime.feature.field_desc.FieldDesc(name=\"testcol\", dtype=fd.DataType.INT64, delimiter=\"\", format=\"\", shape=[10], is_sparse=False, vocabulary=[]))",
		nc.GenPythonCode())

	emd := &EmbeddingColumn{
		CategoryColumn: nil,
		Dimension:      128,
	}
	a.Equal("runtime.feature.column.EmbeddingColumn(category_column=None, dimension=128, combiner=\"\", initializer=\"\", name=\"\")",
		emd.GenPythonCode())
}

func TestFeatureColumnMarshalJSON(t *testing.T) {
	a := assert.New(t)

	nc := &NumericColumn{
		FieldDesc: &FieldDesc{
			Name:  "testcol",
			Shape: []int{10},
			DType: Float,
		},
	}
	s, err := MarshalToJSONString(nc)
	a.NoError(err)
	a.Equal(`{"type":"NumericColumn","value":{"field_desc":{"name":"testcol","dtype":1,"delimiter":"","format":"","shape":[10],"is_sparse":false,"vocabulary":null,"max_id":0}}}`, s)

	emb := &EmbeddingColumn{
		CategoryColumn: &CrossColumn{
			Keys:           []interface{}{"c1", nc},
			HashBucketSize: 64,
		},
		Dimension: 128,
		Name:      "c3",
	}
	s, err = MarshalToJSONString(emb)
	a.NoError(err)
	a.Equal(`{"type":"EmbeddingColumn","value":{"category_column":{"type":"CrossColumn","value":{"hash_bucket_size":64,"keys":["c1",{"type":"NumericColumn","value":{"field_desc":{"name":"testcol","dtype":1,"delimiter":"","format":"","shape":[10],"is_sparse":false,"vocabulary":null,"max_id":0}}}]}},"combiner":"","dimension":128,"initializer":"","name":"c3"}}`, s)
}
