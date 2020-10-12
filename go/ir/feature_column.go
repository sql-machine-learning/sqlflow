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

import (
	"fmt"
	"strings"
)

// FeatureColumn corresponds to the COLUMN clause in TO TRAIN.
type FeatureColumn interface {
	GetFieldDesc() []*FieldDesc
	ApplyTo(*FieldDesc) (FeatureColumn, error)
	GenPythonCode() string
}

// CategoryColumn corresponds to categorical column
type CategoryColumn interface {
	FeatureColumn
	NumClass() int64
}

// FieldDesc describes a field used as the input to a feature column.
type FieldDesc struct {
	Name        string `json:"name"`         // the name for a field, e.g. "petal_length"
	DType       int    `json:"dtype"`        // data type of the values, e.g. "float", "int32"
	DTypeWeight int    `json:"dtype_weight"` // data type of the keys.
	Delimiter   string `json:"delimiter"`    // Needs to be "," if the field saves strings like "1,23,42".
	DelimiterKV string `json:"delimiter_kv"` // k-v list format like k:v-k:v, delimiter:"-", delimiter_kv:":"
	Format      string `json:"format"`       // The data format, "", "csv" or "kv"
	Shape       []int  `json:"shape"`        // [3] if the field saves strings of three numbers like "1,23,42".
	IsSparse    bool   `json:"is_sparse"`    // If the field saves a sparse tensor.
	// Vocabulary stores all possible enumerate values if the column type is string,
	// e.g. the column values are: "MALE", "FEMALE", "NULL"
	Vocabulary map[string]string `json:"vocabulary"` // use a map to generate a list without duplication
	// if the column data is used as embedding(category_column()), the `num_buckets` should use the maxID
	// appeared in the sample data. if error still occurs, users should set `num_buckets` manually.
	MaxID int64
}

// GenPythonCode generate Python code to construct a runtime.feature.field_desc
func (fd *FieldDesc) GenPythonCode() string {
	isSparseStr := "False"
	if fd.IsSparse {
		isSparseStr = "True"
	}
	vocabList := []string{}
	for k := range fd.Vocabulary {
		vocabList = append(vocabList, k)
	}

	var shapeStr string
	if fd.Shape == nil {
		shapeStr = "[]"
	} else {
		shapeStr = AttrToPythonValue(fd.Shape)
	}

	// pass format = "" to let runtime feature derivation to fill it in.
	return fmt.Sprintf(`runtime.feature.field_desc.FieldDesc(name="%s", dtype=runtime.feature.field_desc.DataType.%s, dtype_weight=runtime.feature.field_desc.DataType.%s, delimiter="%s", delimiter_kv="%s", format="", shape=%s, is_sparse=%s, vocabulary=%s)`,
		fd.Name,
		strings.ToUpper(DTypeToString(fd.DType)),
		strings.ToUpper(DTypeToString(fd.DTypeWeight)),
		fd.Delimiter,
		fd.DelimiterKV,
		shapeStr,
		isSparseStr,
		AttrToPythonValue(vocabList),
	)
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
func (c *NumericColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{c.FieldDesc}
}

// ApplyTo applies the FeatureColumn to a new field
func (c *NumericColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	return &NumericColumn{other}, nil
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *NumericColumn) GenPythonCode() string {
	code := fmt.Sprintf(`runtime.feature.column.NumericColumn(%s)`, c.FieldDesc.GenPythonCode())
	return code
}

// BucketColumn represents `tf.feature_column.bucketized_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/bucketized_column
type BucketColumn struct {
	SourceColumn *NumericColumn
	Boundaries   []int
}

// GetFieldDesc returns FieldDesc member
func (c *BucketColumn) GetFieldDesc() []*FieldDesc {
	return c.SourceColumn.GetFieldDesc()
}

// ApplyTo applies the FeatureColumn to a new field
func (c *BucketColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	sourceColumn, err := c.SourceColumn.ApplyTo(other)
	if err != nil {
		return nil, err
	}
	return &BucketColumn{
		SourceColumn: sourceColumn.(*NumericColumn),
		Boundaries:   c.Boundaries,
	}, nil
}

// NumClass returns class number of BucketColumn
func (c *BucketColumn) NumClass() int64 {
	return int64(len(c.Boundaries)) + 1
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *BucketColumn) GenPythonCode() string {
	code := fmt.Sprintf(`runtime.feature.column.BucketColumn(%s, %s)`,
		c.SourceColumn.GenPythonCode(),
		AttrToPythonValue(c.Boundaries),
	)
	return code
}

// CrossColumn represents `tf.feature_column.crossed_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/crossed_column
type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (c *CrossColumn) GetFieldDesc() []*FieldDesc {
	var retKeys []*FieldDesc
	for idx, k := range c.Keys {
		if strKey, ok := k.(string); ok {
			retKeys = append(retKeys, &FieldDesc{
				Name:  strKey,
				DType: String,
				Shape: []int{1},
			})
		} else if _, ok := k.(FeatureColumn); ok {
			retKeys = append(retKeys, c.Keys[idx].(*NumericColumn).GetFieldDesc()[0])
		}
		// k is not possible to be neither string and FeatureColumn, the ir_generator should
		// catch the syntax error.
	}
	return retKeys
}

// ApplyTo applies the FeatureColumn to a new field
func (c *CrossColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	return nil, fmt.Errorf("CrossColumn doesn't support the method ApplyTo")
}

// NumClass returns class number of CrossColumn
func (c *CrossColumn) NumClass() int64 {
	return c.HashBucketSize
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *CrossColumn) GenPythonCode() string {
	keysCode := []string{}
	for _, k := range c.Keys {
		if strKey, ok := k.(string); ok {
			keysCode = append(keysCode, fmt.Sprintf(`"%s"`, strKey))
		} else if nc, ok := k.(*NumericColumn); ok {
			keysCode = append(keysCode, nc.GenPythonCode())
		}
	}
	code := fmt.Sprintf(`runtime.feature.column.CrossColumn([%s], %d)`,
		strings.Join(keysCode, ","),
		c.HashBucketSize,
	)
	return code
}

// CategoryIDColumn represents `tf.feature_column.categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_identity
type CategoryIDColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (c *CategoryIDColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{c.FieldDesc}
}

// ApplyTo applies the FeatureColumn to a new field
func (c *CategoryIDColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	return &CategoryIDColumn{other, c.BucketSize}, nil
}

// NumClass returns class number of CategoryIDColumn
func (c *CategoryIDColumn) NumClass() int64 {
	return c.BucketSize
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *CategoryIDColumn) GenPythonCode() string {
	code := fmt.Sprintf(`runtime.feature.column.CategoryIDColumn(%s, %d)`,
		c.FieldDesc.GenPythonCode(),
		c.BucketSize,
	)
	return code
}

// CategoryHashColumn represents `tf.feature_column.categorical_column_with_hash_bucket`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/categorical_column_with_hash_bucket
type CategoryHashColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (c *CategoryHashColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{c.FieldDesc}
}

// ApplyTo applies the FeatureColumn to a new field
func (c *CategoryHashColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	return &CategoryHashColumn{other, c.BucketSize}, nil
}

// NumClass returns class number of CategoryHashColumn
func (c *CategoryHashColumn) NumClass() int64 {
	return c.BucketSize
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *CategoryHashColumn) GenPythonCode() string {
	code := fmt.Sprintf(`runtime.feature.column.CategoryHashColumn(%s, %d)`,
		c.FieldDesc.GenPythonCode(),
		c.BucketSize,
	)
	return code
}

// SeqCategoryIDColumn represents `tf.feature_column.sequence_categorical_column_with_identity`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/sequence_categorical_column_with_identity
type SeqCategoryIDColumn struct {
	FieldDesc  *FieldDesc
	BucketSize int64
}

// GetFieldDesc returns FieldDesc member
func (c *SeqCategoryIDColumn) GetFieldDesc() []*FieldDesc {
	return []*FieldDesc{c.FieldDesc}
}

// ApplyTo applies the FeatureColumn to a new field
func (c *SeqCategoryIDColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	return &SeqCategoryIDColumn{other, c.BucketSize}, nil
}

// NumClass returns class number of SeqCategoryIDColumn
func (c *SeqCategoryIDColumn) NumClass() int64 {
	return c.BucketSize
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *SeqCategoryIDColumn) GenPythonCode() string {
	code := fmt.Sprintf(`runtime.feature.column.SeqCategoryIDColumn(%s, %d)`,
		c.FieldDesc.GenPythonCode(),
		c.BucketSize,
	)
	return code
}

// EmbeddingColumn represents `tf.feature_column.embedding_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/embedding_column
type EmbeddingColumn struct {
	CategoryColumn
	Dimension   int
	Combiner    string
	Initializer string
	// only used when EMBEDDING(col_name, ...) this will set CategoryColumn = nil
	// will fill the feature column details using feature_derivation
	Name string
}

// GetFieldDesc returns FieldDesc member
func (c *EmbeddingColumn) GetFieldDesc() []*FieldDesc {
	if c.CategoryColumn == nil {
		return []*FieldDesc{}
	}
	return c.CategoryColumn.GetFieldDesc()
}

// ApplyTo applies the FeatureColumn to a new field
func (c *EmbeddingColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	ret := &EmbeddingColumn{
		Dimension:   c.Dimension,
		Combiner:    c.Combiner,
		Initializer: c.Initializer,
		Name:        other.Name,
	}
	if c.CategoryColumn != nil {
		var err error
		fc, err := c.CategoryColumn.ApplyTo(other)
		if err != nil {
			return nil, err
		}

		if categoryFc, ok := fc.(CategoryColumn); !ok {
			ret.CategoryColumn = categoryFc
		} else {
			return nil, fmt.Errorf("Embedding.ApplyTo should return CategoryColumn")
		}
	}
	return ret, nil
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *EmbeddingColumn) GenPythonCode() string {
	catColCode := ""
	if c.CategoryColumn == nil {
		catColCode = "None"
	} else {
		catColCode = c.CategoryColumn.GenPythonCode()
	}
	code := fmt.Sprintf(`runtime.feature.column.EmbeddingColumn(category_column=%s, dimension=%d, combiner="%s", initializer="%s", name="%s")`,
		catColCode,
		c.Dimension,
		c.Combiner,
		c.Initializer,
		c.Name,
	)
	return code
}

// IndicatorColumn represents `tf.feature_column.indicator_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/indicator_column
type IndicatorColumn struct {
	CategoryColumn
	// only used when INDICATOR(col_name, ...) this will set CategoryColumn = nil
	// will fill the feature column details using feature_derivation
	Name string
}

// GetFieldDesc returns FieldDesc member
func (c *IndicatorColumn) GetFieldDesc() []*FieldDesc {
	if c.CategoryColumn == nil {
		return []*FieldDesc{}
	}
	return c.CategoryColumn.GetFieldDesc()
}

// ApplyTo applies the FeatureColumn to a new field
func (c *IndicatorColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	ret := &IndicatorColumn{Name: other.Name}
	if c.CategoryColumn != nil {
		fc, err := c.CategoryColumn.ApplyTo(other)
		if err != nil {
			return nil, err
		}

		if catColumn, ok := fc.(CategoryColumn); ok {
			ret.CategoryColumn = catColumn
		} else {
			return nil, fmt.Errorf("Indicator.ApplyTo should return CategoryColumn")
		}
	}
	return ret, nil
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *IndicatorColumn) GenPythonCode() string {
	catColCode := ""
	if c.CategoryColumn == nil {
		catColCode = "None"
	} else {
		catColCode = c.CategoryColumn.GenPythonCode()
	}
	code := fmt.Sprintf(`runtime.feature.column.IndicatorColumn(category_column=%s, name="%s")`,
		catColCode,
		c.Name,
	)
	return code
}

// WeightedCategoryColumn represents `tf.feature_column.weighted_categorical_column`
// ref: https://www.tensorflow.org/api_docs/python/tf/feature_column/weighted_categorical_column
type WeightedCategoryColumn struct {
	CategoryColumn
	// only used when WEIGHTED_CATEGORY(col_name, ...) this will set CategoryColumn = nil
	// will fill the feature column details using feature_derivation
	Name string
}

// GetFieldDesc returns FieldDesc member
func (c *WeightedCategoryColumn) GetFieldDesc() []*FieldDesc {
	if c.CategoryColumn == nil {
		return []*FieldDesc{}
	}
	return c.CategoryColumn.GetFieldDesc()
}

// NumClass returns class number of CategoryIDColumn
func (c *WeightedCategoryColumn) NumClass() int64 {
	return c.CategoryColumn.NumClass()
}

// ApplyTo applies the FeatureColumn to a new field
func (c *WeightedCategoryColumn) ApplyTo(other *FieldDesc) (FeatureColumn, error) {
	ret := &WeightedCategoryColumn{Name: other.Name}
	if c.CategoryColumn != nil {
		fc, err := c.CategoryColumn.ApplyTo(other)
		if err != nil {
			return nil, err
		}

		if catColumn, ok := fc.(CategoryColumn); ok {
			ret.CategoryColumn = catColumn
		} else {
			return nil, fmt.Errorf("WeightedCategoryColumn.ApplyTo should return CategoryColumn")
		}
	}
	return ret, nil
}

// GenPythonCode generate Python code to construct a runtime.feature.column.*
func (c *WeightedCategoryColumn) GenPythonCode() string {
	catColCode := ""
	if c.CategoryColumn == nil {
		catColCode = "None"
	} else {
		catColCode = c.CategoryColumn.GenPythonCode()
	}
	// FIXME(typhoonzero): add runtime.feature.column.WeightedCategoryColumn
	code := fmt.Sprintf(`runtime.feature.column.WeightedCategoryColumn(category_column=%s, name="%s")`,
		catColCode,
		c.Name,
	)
	return code
}
