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
	"strconv"
)

type categoryIDColumn struct {
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
}

type sequenceCategoryIDColumn struct {
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
	IsSequence bool
}

func (cc *categoryIDColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize), nil
}

func (cc *categoryIDColumn) GetDelimiter() string {
	return cc.Delimiter
}

func (cc *categoryIDColumn) GetDtype() string {
	return cc.Dtype
}

func (cc *categoryIDColumn) GetKey() string {
	return cc.Key
}

func (cc *categoryIDColumn) GetInputShape() string {
	return fmt.Sprintf("[%d]", cc.BucketSize)
}

func (cc *categoryIDColumn) GetColumnType() int {
	return columnTypeCategoryID
}

func (cc *sequenceCategoryIDColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize), nil
}

func (cc *sequenceCategoryIDColumn) GetDelimiter() string {
	return cc.Delimiter
}

func (cc *sequenceCategoryIDColumn) GetDtype() string {
	return cc.Dtype
}

func (cc *sequenceCategoryIDColumn) GetKey() string {
	return cc.Key
}

func (cc *sequenceCategoryIDColumn) GetInputShape() string {
	return fmt.Sprintf("[%d]", cc.BucketSize)
}

func (cc *sequenceCategoryIDColumn) GetColumnType() int {
	return columnTypeSeqCategoryID
}

func parseCategoryColumnKey(el *exprlist) (*columnSpec, error) {
	if (*el)[1].typ == 0 {
		// explist, maybe DENSE/SPARSE expressions
		subExprList := (*el)[1].sexp
		isSparse := subExprList[0].val == sparse
		return resolveColumnSpec(&subExprList, isSparse)
	}
	return nil, nil
}

func resolveSeqCategoryIDColumn(el *exprlist) (*sequenceCategoryIDColumn, *columnSpec, error) {
	key, bucketSize, delimiter, cs, err := parseCategoryIDColumnExpr(el)
	if err != nil {
		return nil, nil, err
	}
	return &sequenceCategoryIDColumn{
		Key:        key,
		BucketSize: bucketSize,
		Delimiter:  delimiter,
		// TODO(typhoonzero): support config dtype
		Dtype:      "int64",
		IsSequence: true}, cs, nil
}

func resolveCategoryIDColumn(el *exprlist) (*categoryIDColumn, *columnSpec, error) {
	key, bucketSize, delimiter, cs, err := parseCategoryIDColumnExpr(el)
	if err != nil {
		return nil, nil, err
	}
	return &categoryIDColumn{
		Key:        key,
		BucketSize: bucketSize,
		Delimiter:  delimiter,
		// TODO(typhoonzero): support config dtype
		Dtype: "int64"}, cs, nil
}

func parseCategoryIDColumnExpr(el *exprlist) (string, int, string, *columnSpec, error) {
	if len(*el) != 3 && len(*el) != 4 {
		return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID expression format: %s", *el)
	}
	var cs *columnSpec
	key := ""
	if (*el)[1].typ == 0 {
		// explist, maybe DENSE/SPARSE expressions
		subExprList := (*el)[1].sexp
		isSparse := subExprList[0].val == sparse
		cs, err := resolveColumnSpec(&subExprList, isSparse)
		if err != nil {
			return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID expression format: %s", *el)
		}
		key = cs.ColumnName
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID key: %s, err: %s", (*el)[1], err)
		}
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	delimiter := ""
	if len(*el) == 4 {
		delimiter, err = resolveDelimiter((*el)[3].val)
		if err != nil {
			return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID delimiter: %s, %s", (*el)[3].val, err)
		}
	}
	return key, bucketSize, delimiter, cs, nil
}
