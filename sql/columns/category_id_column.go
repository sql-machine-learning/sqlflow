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
)

// CategoryIDColumn is the wrapper of `tf.feature_column.categorical_column_with_identity`
type CategoryIDColumn struct {
	FeatureColumnMetasImpl
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
}

// SequenceCategoryIDColumn is the wrapper of `tf.feature_column.sequence_categorical_column_with_identity`
// NOTE: only used in tf >= 2.0 versions.
type SequenceCategoryIDColumn struct {
	FeatureColumnMetasImpl
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
}

// GenerateCode implements the FeatureColumn interface.
func (cc *CategoryIDColumn) GenerateCode(cs *FieldMeta) ([]string, error) {
	return []string{fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize)}, nil
}

// GetKey implements the FeatureColumn interface.
func (cc *CategoryIDColumn) GetKey() string {
	return cc.Key
}

// GetColumnType implements the FeatureColumn interface.
func (cc *CategoryIDColumn) GetColumnType() int {
	return ColumnTypeCategoryID
}

// GenerateCode implements the FeatureColumn interface.
func (cc *SequenceCategoryIDColumn) GenerateCode(cs *FieldMeta) ([]string, error) {
	return []string{fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize)}, nil
}

// GetKey implements the FeatureColumn interface.
func (cc *SequenceCategoryIDColumn) GetKey() string {
	return cc.Key
}

// GetColumnType implements the FeatureColumn interface.
func (cc *SequenceCategoryIDColumn) GetColumnType() int {
	return ColumnTypeSeqCategoryID
}
