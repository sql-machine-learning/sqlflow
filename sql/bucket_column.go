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

type bucketColumn struct {
	SourceColumn *numericColumn
	Boundaries   []int
}

func (bc *bucketColumn) GenerateCode() (string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode()
	return fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode,
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ",")), nil
}

func (bc *bucketColumn) GetDelimiter() string {
	return ""
}

func (bc *bucketColumn) GetDtype() string {
	return ""
}

func (bc *bucketColumn) GetKey() string {
	return bc.SourceColumn.Key
}

func (bc *bucketColumn) GetInputShape() string {
	return bc.SourceColumn.GetInputShape()
}

func (bc *bucketColumn) GetColumnType() int {
	return columnTypeBucket
}

func resolveBucketColumn(el *exprlist) (*bucketColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
	}
	sourceExprList := (*el)[1]
	boundariesExprList := (*el)[2]
	source, _, err := resolveColumn(&sourceExprList.sexp)
	if err != nil {
		return nil, err
	}
	if source.GetColumnType() != columnTypeNumeric {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
	}
	boundaries, err := resolveLispExpression(boundariesExprList)
	if err != nil {
		return nil, err
	}
	if _, ok := boundaries.([]interface{}); !ok {
		return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
	}
	b, err := transformToIntList(boundaries.([]interface{}))
	if err != nil {
		return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
	}
	return &bucketColumn{
		// SourceColumn: source.(*numericColumn),
		SourceColumn: source.(*numericColumn),
		Boundaries:   b}, nil
}
