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

package ir

// NumericColumn represents a dense tensor for the model input
//
// FieldMeta indicates the meta information for decoding the field. Please be aware
// that FieldMeta also contains information for dimension and data type
type NumericColumn struct {
	FieldMeta *FieldMeta
}

// GetFieldMeta returns FieldMeta member
func (nc *NumericColumn) GetFieldMeta() []*FieldMeta {
	return []*FieldMeta{nc.FieldMeta}
}

// BucketColumn represents `tf.feature_column.bucketized_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/bucketized_column
type BucketColumn struct {
	SourceColumn *NumericColumn
	Boundaries   []int
}

// GetFieldMeta returns FieldMeta member
func (bc *BucketColumn) GetFieldMeta() []*FieldMeta {
	return bc.SourceColumn.GetFieldMeta()
}

// CrossColumn represents `tf.feature_column.crossed_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column
type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

// GetFieldMeta returns FieldMeta member
func (cc *CrossColumn) GetFieldMeta() []*FieldMeta {
	var retKeys []*FieldMeta
	for idx, k := range cc.Keys {
		if _, ok := k.(string); ok {
			continue
		} else if _, ok := k.(FeatureColumn); ok {
			retKeys = append(retKeys, cc.Keys[idx].(*NumericColumn).GetFieldMeta()[0])
		}
		// k is not possible to be neither string and FeatureColumn, the ir_generator should
		// catch the syntax error.
	}
	return retKeys
}

// CategoryIDColumn represents `tf.feature_column.categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_identity
type CategoryIDColumn struct {
	FieldMeta  *FieldMeta
	BucketSize int64
}

// GetFieldMeta returns FieldMeta member
func (cc *CategoryIDColumn) GetFieldMeta() []*FieldMeta {
	return []*FieldMeta{cc.FieldMeta}
}

// SeqCategoryIDColumn represents `tf.feature_column.sequence_categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/sequence_categorical_column_with_identity
type SeqCategoryIDColumn struct {
	FieldMeta  *FieldMeta
	BucketSize int
}

// GetFieldMeta returns FieldMeta member
func (scc *SeqCategoryIDColumn) GetFieldMeta() []*FieldMeta {
	return []*FieldMeta{scc.FieldMeta}
}

// EmbeddingColumn represents `tf.feature_column.embedding_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/embedding_column
type EmbeddingColumn struct {
	CategoryColumn interface{}
	Dimension      int
	Combiner       string
	Initializer    string
	// only used when EMBEDDING(col_name, ...) this will set CategoryColumn = nil
	// will fill the feature column details using feature_derivation
	Name string
}

// GetFieldMeta returns FieldMeta member
func (ec *EmbeddingColumn) GetFieldMeta() []*FieldMeta {
	if ec.CategoryColumn == nil {
		return []*FieldMeta{}
	}
	return ec.CategoryColumn.(FeatureColumn).GetFieldMeta()
}
