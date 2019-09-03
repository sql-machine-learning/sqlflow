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

package columns

import (
	"fmt"
	"strings"
)

// FeatureMap only used by codegen_alps, a table containing column parse
// informations.
type FeatureMap struct {
	Table     string
	Partition string
}

// ColumnSpec defines how to generate codes to parse column data to tensor/sparsetensor
type ColumnSpec struct {
	ColumnName string
	IsSparse   bool
	Shape      []int
	DType      string
	Delimiter  string
	FeatureMap FeatureMap
}

// ToString generates the debug string of ColumnSpec
func (cs *ColumnSpec) ToString() string {
	if cs.IsSparse {
		shape := strings.Join(strings.Split(fmt.Sprint(cs.Shape), " "), ",")
		if len(cs.Shape) > 1 {
			groupCnt := len(cs.Shape)
			return fmt.Sprintf("GroupedSparseColumn(name=\"%s\", shape=%s, dtype=\"%s\", group=%d, group_separator='\\002')",
				cs.ColumnName, shape, cs.DType, groupCnt)
		}
		return fmt.Sprintf("SparseColumn(name=\"%s\", shape=%s, dtype=\"%s\")", cs.ColumnName, shape, cs.DType)

	}
	return fmt.Sprintf("DenseColumn(name=\"%s\", shape=%s, dtype=\"%s\", separator=\"%s\")",
		cs.ColumnName,
		strings.Join(strings.Split(fmt.Sprint(cs.Shape), " "), ","),
		cs.DType,
		cs.Delimiter)
}
