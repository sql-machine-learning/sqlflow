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

package codegen

// NumericColumn represents a dense tensor for the model input
//
// FieldMeta indicates the meta information for decoding the field. Please be aware
// that FieldMeta also contains information for dimension and data type
type NumericColumn struct {
	FieldMeta *FieldMeta
}

func (NumericColumn) isFeatureColumn() {}

// BucketColumn represents `tf.feature_column.bucketized_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/bucketized_column
type BucketColumn struct {
	SourceColumn *NumericColumn
	Boundaries   []int
}

func (BucketColumn) isFeatureColumn() {}

// CrossColumn represents `tf.feature_column.crossed_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column
type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

func (CrossColumn) isFeatureColumn() {}

// CategoryIDColumn represents `tf.feature_column.categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_identity
type CategoryIDColumn struct {
	FieldMeta  *FieldMeta
	BucketSize int
}

func (CategoryIDColumn) isFeatureColumn() {}

// SeqCategoryIDColumn represents `tf.feature_column.sequence_categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/sequence_categorical_column_with_identity
type SeqCategoryIDColumn struct {
	FieldMeta  *FieldMeta
	BucketSize int
}

func (SeqCategoryIDColumn) isFeatureColumn() {}

// EmbeddingColumn represents `tf.feature_column.embedding_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/embedding_column
type EmbeddingColumn struct {
	CategoryColumn interface{}
	Dimension      int
	Combiner       string
	Initializer    string
}

func (EmbeddingColumn) isFeatureColumn() {}
