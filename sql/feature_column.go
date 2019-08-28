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

package sql

import (
	"fmt"
	"strings"
)

const (
	columnTypeBucket        = 0
	columnTypeEmbedding     = 1
	columnTypeNumeric       = 2
	columnTypeCategoryID    = 3
	columnTypeSeqCategoryID = 3
	columnTypeCross         = 4
)

// featureColumn is an interface that all types of feature columns
// should follow. featureColumn is used to generate feature column code.
type featureColumn interface {
	GenerateCode() (string, error)
	// FIXME(typhoonzero): remove delimiter, dtype shape from feature column
	// get these from column spec claused or by feature derivation.
	GetDelimiter() string
	GetDtype() string
	GetKey() string
	// return input shape json string, like "[2,3]"
	GetInputShape() string
	GetColumnType() int
}

// resolveFeatureColumn returns the acutal feature column typed struct
// as well as the columnSpec infomation.
func resolveColumn(el *exprlist) (featureColumn, *columnSpec, error) {
	head := (*el)[0].val
	if head == "" {
		return nil, nil, fmt.Errorf("column description expects format like NUMERIC(key) etc, got %v", el)
	}

	switch strings.ToUpper(head) {
	case dense:
		cs, err := resolveColumnSpec(el, false)
		return nil, cs, err
	case sparse:
		cs, err := resolveColumnSpec(el, true)
		return nil, cs, err
	case numeric:
		// TODO(typhoonzero): support NUMERIC(DENSE(col)) and NUMERIC(SPARSE(col))
		fc, err := resolveNumericColumn(el)
		return fc, nil, err
	case bucket:
		fc, err := resolveBucketColumn(el)
		return fc, nil, err
	case cross:
		fc, err := resolveCrossColumn(el)
		return fc, nil, err
	case categoryID:
		return resolveCategoryIDColumn(el)
	case seqCategoryID:
		return resolveSeqCategoryIDColumn(el)
	case embedding:
		fc, err := resolveEmbeddingColumn(el)
		return fc, nil, err
	default:
		return nil, nil, fmt.Errorf("not supported expr: %s", head)
	}
}
