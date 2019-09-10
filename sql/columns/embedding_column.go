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

// EmbeddingColumn is the wrapper of `tf.feature_column.embedding_column`
type EmbeddingColumn struct {
	FeatureColumnMetasImpl
	Key            string // only used when CategoryColumn = nil, feature derivation will fill up the details
	CategoryColumn interface{}
	Dimension      int
	Combiner       string
	Initializer    string
}

// GetKey implements the FeatureColumn interface.
func (ec *EmbeddingColumn) GetKey() string {
	if ec.CategoryColumn != nil {
		return ec.CategoryColumn.(FeatureColumn).GetKey()
	}
	return ec.Key
}

// GetColumnType implements the FeatureColumn interface.
func (ec *EmbeddingColumn) GetColumnType() int {
	return ColumnTypeEmbedding
}

// GenerateCode implements the FeatureColumn interface.
func (ec *EmbeddingColumn) GenerateCode(cs *FieldMeta) ([]string, error) {
	catColumn, ok := ec.CategoryColumn.(FeatureColumn)
	if !ok {
		return []string{}, fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
	}
	sourceCode, err := catColumn.GenerateCode(cs)
	if err != nil {
		return []string{}, err
	}
	if len(sourceCode) > 1 {
		return []string{}, fmt.Errorf("does not support grouped column: %v", sourceCode)
	}
	return []string{fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
		sourceCode[0], ec.Dimension, ec.Combiner)}, nil
}
