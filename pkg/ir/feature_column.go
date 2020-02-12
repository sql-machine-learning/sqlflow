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

// FeatureColumn corresponds to the COLUMN clause in TO TRAIN.
type FeatureColumn interface {
	GetFieldDesc() []*FieldDesc
}

// FieldDesc describes a field used as the input to a feature column.
type FieldDesc struct {
	Name      string `json:"name"`      // the name for a field, e.g. "petal_length"
	DType     int    `json:"dtype"`     // e.g. "float", "int32"
	Delimiter string `json:"delimiter"` // Needs to be "," if the field saves strings like "1,23,42".
	Shape     []int  `json:"shape"`     // [3] if the field saves strings of three numbers like "1,23,42".
	IsSparse  bool   `json:"is_sparse"` // If the field saves a sparse tensor.
	// Vocabulary stores all possible enumerate values if the column type is string,
	// e.g. the column values are: "MALE", "FEMALE", "NULL"
	Vocabulary map[string]string `json:"vocabulary"` // use a map to generate a list without duplication
	// if the column data is used as embedding(category_column()), the `num_buckets` should use the maxID
	// appeared in the sample data. if error still occurs, users should set `num_buckets` manually.
	MaxID int64
}

// Possible DType values in FieldDesc
const (
	Int int = iota
	Float
	String
)

// NumericColumn represents a dense tensor for the model input
//
// FieldDesc indicates the meta information for decoding the field. Please be aware
// that FieldDesc also contains information for dimension and data type
type NumericColumn struct {
	FieldDesc *FieldDesc
}

// GetFieldDesc returns FieldDesc member
func (nc *NumericColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{nc.FieldDesc}
}

// BucketColumn represents `tf.feature_column.bucketized_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/bucketized_column
type BucketColumn struct {
	SourceColumn *NumericColumn
	Boundaries   []int
}

// GetFieldDesc returns FieldDesc member
func (bc *BucketColumn) GetFieldDesc() []*FieldDesc {
	return bc.SourceColumn.GetFieldDesc()
}

// CrossColumn represents `tf.feature_column.crossed_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column
type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

// GetFieldDesc returns FieldDesc member
func (cc *CrossColumn) GetFieldDesc() []*FieldDesc {
	var retKeys []*FieldDesc
	for idx, k := range cc.Keys {
		if _, ok := k.(string); ok {
			continue
		} else if _, ok := k.(FeatureColumn); ok {
			retKeys = append(retKeys, cc.Keys[idx].(*NumericColumn).GetFieldDesc()[0])
		}
		// k is not possible to be neither string and FeatureColumn, the ir_generator should
		// catch the syntax error.
	}
	return retKeys
}

// CategoryIDColumn represents `tf.feature_column.categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_identity
type CategoryIDColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (cc *CategoryIDColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{cc.FieldDesc}
}

// CategoryHashColumn represents `tf.feature_column.categorical_column_with_hash_bucket`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_hash_bucket
type CategoryHashColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (cc *CategoryHashColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{cc.FieldDesc}
}

// SeqCategoryIDColumn represents `tf.feature_column.sequence_categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/sequence_categorical_column_with_identity
type SeqCategoryIDColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int
}

// GetFieldDesc returns FieldDesc member
func (scc *SeqCategoryIDColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{scc.FieldDesc}
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

// GetFieldDesc returns FieldDesc member
func (ec *EmbeddingColumn) GetFieldDesc() []*FieldDesc {
	if ec.CategoryColumn == nil {
		return []*FieldDesc{}
	}
	return ec.CategoryColumn.(FeatureColumn).GetFieldDesc()
}
