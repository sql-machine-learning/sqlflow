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

// NumericColumn is the wrapper of `tf.feature_column.numeric_column`
type NumericColumn struct {
	FeatureColumnMetasImpl
	Key string
}

// GenerateCode implements FeatureColumn interface.
func (nc *NumericColumn) GenerateCode(cs *FieldMeta) ([]string, error) {
	var shape []int
	if len(nc.FieldMetas) > 0 {
		shape = nc.FieldMetas[0].Shape
	} else {
		shape = []int{1}
	}
	return []string{fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)", nc.Key,
		strings.Join(strings.Split(fmt.Sprint(shape), " "), ","))}, nil
}

// GetKey implements FeatureColumn interface.
func (nc *NumericColumn) GetKey() string {
	return nc.Key
}

// GetColumnType implements FeatureColumn interface.
func (nc *NumericColumn) GetColumnType() int {
	return ColumnTypeNumeric
}
