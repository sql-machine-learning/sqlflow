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

const (
	// ColumnTypeBucket is the `FeatureColumn` of type bucket_column
	ColumnTypeBucket = 0
	// ColumnTypeEmbedding is the `FeatureColumn` of type embedding_column
	ColumnTypeEmbedding = 1
	// ColumnTypeNumeric is the `FeatureColumn` of type numeric_column
	ColumnTypeNumeric = 2
	// ColumnTypeCategoryID is the `FeatureColumn` of type category_id_column
	ColumnTypeCategoryID = 3
	// ColumnTypeSeqCategoryID is the `FeatureColumn` of type sequence_category_id_column
	ColumnTypeSeqCategoryID = 4
	// ColumnTypeCross is the `FeatureColumn` of type cross_column
	ColumnTypeCross = 5
)

// FeatureColumn is an interface that all types of feature columns
// should follow. featureColumn is used to generate feature column code.
type FeatureColumn interface {
	FeatureColumnMetas
	// NOTE: submitters need to know the FieldMeta when generating
	// feature_column code. And we maybe use one compound column's data to generate
	// multiple feature columns, so return a list of strings.
	GenerateCode(cs *FieldMeta) ([]string, error)
	GetKey() string

	// FIXME(typhoonzero): remove delimiter, dtype shape from feature column
	// get these from column spec claused or by feature derivation.
	// GetDelimiter() string
	// GetDtype() string
	// GetInputShape() string

	GetColumnType() int
}

// FeatureColumnMetas is the interface of FieldMetas list, embeded in FeatureColumn interface.
type FeatureColumnMetas interface {
	GetFieldMetas() []*FieldMeta
	AppendFieldMetas(fm *FieldMeta)
}

// FeatureColumnMetasImpl is a general struct that all feature column structs should embed.
type FeatureColumnMetasImpl struct {
	// one feature column may use more than one field as input data, like cross_column
	FieldMetas []*FieldMeta
}

// GetFieldMetas returns the FieldMeta List
func (fcm *FeatureColumnMetasImpl) GetFieldMetas() []*FieldMeta {
	return fcm.FieldMetas
}

// AppendFieldMetas append a new FieldMeta
func (fcm *FeatureColumnMetasImpl) AppendFieldMetas(fm *FieldMeta) {
	fcm.FieldMetas = append(fcm.FieldMetas, fm)
}
