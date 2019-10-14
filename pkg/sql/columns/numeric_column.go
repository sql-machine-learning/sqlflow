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
	"encoding/json"
	"fmt"
	"strings"
)

// NumericColumn is the wrapper of `tf.feature_column.numeric_column`
type NumericColumn struct {
	Key       string
	Shape     []int
	Delimiter string
	Dtype     string
}

// GenerateCode implements FeatureColumn interface.
func (nc *NumericColumn) GenerateCode(cs *ColumnSpec) ([]string, error) {
	return []string{fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)", nc.Key,
		strings.Join(strings.Split(fmt.Sprint(nc.Shape), " "), ","))}, nil
}

// GetDelimiter implements FeatureColumn interface.
func (nc *NumericColumn) GetDelimiter() string {
	return nc.Delimiter
}

// GetDtype implements FeatureColumn interface.
func (nc *NumericColumn) GetDtype() string {
	return nc.Dtype
}

// GetKey implements FeatureColumn interface.
func (nc *NumericColumn) GetKey() string {
	return nc.Key
}

// GetInputShape implements FeatureColumn interface.
func (nc *NumericColumn) GetInputShape() string {
	jsonBytes, err := json.Marshal(nc.Shape)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// GetColumnType implements FeatureColumn interface.
func (nc *NumericColumn) GetColumnType() int {
	return ColumnTypeNumeric
}
