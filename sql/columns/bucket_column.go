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

// BucketColumn is the wrapper of `tf.feature_column.bucketized_column`
type BucketColumn struct {
	FeatureColumnMetasImpl
	SourceColumn *NumericColumn
	Boundaries   []int
}

// GenerateCode implements the FeatureColumn interface.
func (bc *BucketColumn) GenerateCode(cs *FieldMeta) ([]string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode(cs)
	if len(sourceCode) > 1 {
		return []string{}, fmt.Errorf("does not support grouped column: %v", sourceCode)
	}
	return []string{fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode[0],
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ","))}, nil
}

// GetKey implements the FeatureColumn interface.
func (bc *BucketColumn) GetKey() string {
	return bc.SourceColumn.Key
}

// GetColumnType implements the FeatureColumn interface.
func (bc *BucketColumn) GetColumnType() int {
	return ColumnTypeBucket
}
