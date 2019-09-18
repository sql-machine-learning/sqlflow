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
	"strings"

	"github.com/sql-machine-learning/sqlflow/sql/codegen"
	"github.com/sql-machine-learning/sqlflow/sql/columns"
)

func generateTrainIR(slct *extendedSelect, connStr string) (*codegen.TrainIR, error) {
	tc := slct.trainClause
	estimator := tc.estimator
	attrList, err := generateAttributeIR(&slct.trainAttrs)
	if err != nil {
		return nil, err
	}
	fcMap := make(map[string][]codegen.FeatureColumn)
	for target, columnList := range tc.columns {
		fcList := []codegen.FeatureColumn{}
		for _, colExpr := range columnList {
			if colExpr.typ != 0 {
				// column identifier like "COLUMN a1,b1"
				nc := &codegen.NumericColumn{
					FieldMeta: &codegen.FieldMeta{
						Name:      colExpr.val,
						Shape:     []int{1},
						DType:     codegen.Float,
						IsSparse:  false,
						Delimiter: "",
					}}
				fcList = append(fcList, nc)
			} else {
				fc, err := parseFeatureColumn(&colExpr.sexp)
				if err != nil {
					return nil, fmt.Errorf("parse FeatureColumn failed: %v", err)
				}
				fcList = append(fcList, fc)
			}
		}
		fcMap[target] = fcList
	}
	label := &codegen.NumericColumn{
		FieldMeta: &codegen.FieldMeta{
			Name: tc.label,
		}}

	// TODO(typhoonzero): fill in ValidationSelect when VALIDATE clause is ready
	return &codegen.TrainIR{
		DataSource:       connStr,
		Select:           slct.standardSelect.String(),
		ValidationSelect: "",
		Estimator:        estimator,
		Attributes:       attrList,
		Features:         fcMap,
		Label:            label,
	}, nil
}

func generateAttributeIR(attrs *attrs) ([]*codegen.Attribute, error) {
	ret := []*codegen.Attribute{}
	for k, v := range *attrs {
		resolvedExpr, err := parseExpression(v)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &codegen.Attribute{
			Key:   k,
			Value: resolvedExpr,
		})
	}
	return ret, nil
}

// -------------------------- expression parsers --------------------------------------

// parseExpression is a recursive function, it returns the actual value
// or struct the expression stands for, e.g.
// 1                 ->  int(1)
// "string"          ->  string("string")
// [1,2,3]           ->  []int{1,2,3}
// NUMERIC(c1)       ->  &codegen.NumericColumn{Key: "c1"...}
// [NUMERIC(c1), c2] ->  [&codegen.NumericColumn{Key: "c1"...}, string("c2")]
//
// parameter e could be type `*expr` or `*exprlist` for recursive call.
func parseExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.typ != 0 {
			// TODO(typhoonzero): infer the element expression type like int, float, string
			return expr.val, nil
		}
		return parseExpression(&expr.sexp)
	}
	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of parseExpression must be `expr` or `exprlist` given %s", e)
	}

	headTyp := (*el)[0].typ
	if headTyp == IDENT {
		// expression is a function call format like `NUMERIC(c1)`
		return parseFeatureColumn(el)
	} else if headTyp == '[' {
		// expression is a list of things
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.sexp == nil {
					intVal, err := strconv.Atoi(expr.val)
					// TODO(typhoonzero): support list of float etc.
					if err != nil {
						list = append(list, expr.val)
					} else {
						list = append(list, intVal)
					}
				} else {
					value, err := parseExpression(&expr.sexp)
					if err != nil {
						return nil, err
					}
					list = append(list, value)
				}
			}
		}
		return list, nil
	}
	return nil, fmt.Errorf("not supported expr: %v", el)
}

func parseFeatureColumn(el *exprlist) (codegen.FeatureColumn, error) {
	head := (*el)[0].val
	if head == "" {
		return nil, fmt.Errorf("column description expects format like NUMERIC(key) etc, got %v", el)
	}

	switch strings.ToUpper(head) {
	// case dense:
	// 	cs, err := resolveColumnSpec(el, false)
	// 	return nil, cs, err
	// case sparse:
	// 	cs, err := resolveColumnSpec(el, true)
	// 	return nil, cs, err
	case numeric:
		return parseNumericColumn(el)
	case bucket:
		return parseBucketColumn(el)
	case cross:
		return parseCrossColumn(el)
	case categoryID:
		return parseCategoryIDColumn(el)
	case seqCategoryID:
		return parseSeqCategoryIDColumn(el)
	case embedding:
		return parseEmbeddingColumn(el)
	default:
		return nil, fmt.Errorf("not supported expr: %s", head)
	}
}

func parseNumericColumn(el *exprlist) (*codegen.NumericColumn, error) {
	help := "NUMERIC(DENSE(col_name,...)[, SHAPE])"
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad NUMERIC expression format: %s, should be like: %s", *el, help)
	}
	// TODO(typhoonzero): support DENSE()/SPARSE() here
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s, should be like: %s", (*el)[1], err, help)
	}
	shape, err := parseShape((*el)[2])

	return &codegen.NumericColumn{
		FieldMeta: &codegen.FieldMeta{
			Name:     key,
			DType:    codegen.Float, // default use float dtype if no DENSE()/SPARSE() provided
			Shape:    shape,
			IsSparse: false,
		},
	}, nil
}

func parseBucketColumn(el *exprlist) (*codegen.BucketColumn, error) {
	help := "BUCKET(NUMERIC(...), BOUNDARIES)"
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad BUCKET expression format: %s, should be like: %s", *el, help)
	}

	sourceExprList := (*el)[1]
	boundariesExprList := (*el)[2]
	if sourceExprList.typ != 0 {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %v", sourceExprList)
	}
	source, err := parseFeatureColumn(&sourceExprList.sexp)
	if err != nil {
		return nil, err
	}
	if _, ok := source.(*codegen.NumericColumn); !ok {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
	}
	b, err := parseShape(boundariesExprList)
	if err != nil {
		return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
	}
	return &codegen.BucketColumn{
		SourceColumn: source.(*codegen.NumericColumn),
		Boundaries:   b}, nil
}

func parseCrossColumn(el *exprlist) (*codegen.CrossColumn, error) {
	help := "CROSS([column_1, column_2], HASH_BUCKET_SIZE)"
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad CROSS expression format: %s, should be like: %s", *el, help)
	}

	keysExpr := (*el)[1]
	key, err := parseExpression(keysExpr)
	if err != nil {
		return nil, err
	}
	if _, ok := key.([]interface{}); !ok {
		return nil, fmt.Errorf("bad CROSS expression format: %s, should be like: %s", *el, help)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &codegen.CrossColumn{
		Keys:           key.([]interface{}),
		HashBucketSize: bucketSize}, nil
}

func parseCategoryIDColumn(el *exprlist) (*codegen.CategoryIDColumn, error) {
	help := "CATEGORY_ID(column_1, BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	if (*el)[1].typ == 0 {
		// TODO(typhoonzero): support DENSE()/SPARSE() in category_id_column to fill FieldMeta
		return nil, fmt.Errorf("bad CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID key: %s, err: %s", (*el)[1], err)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &codegen.CategoryIDColumn{
		// TODO(typhoonzero): support CATEGORY_ID(DENSE(...)) to fill FieldMeta
		FieldMeta: &codegen.FieldMeta{
			Name:     key,
			DType:    codegen.Int,
			IsSparse: false,
		},
		BucketSize: bucketSize,
	}, nil
}

func parseSeqCategoryIDColumn(el *exprlist) (*codegen.SeqCategoryIDColumn, error) {
	help := "SEQ_CATEGORY_ID(column_1, BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	if (*el)[1].typ == 0 {
		// TODO(typhoonzero): support DENSE()/SPARSE() in seq_category_id_column to fill FieldMeta
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID key: %s, err: %s", (*el)[1], err)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &codegen.SeqCategoryIDColumn{
		// TODO(typhoonzero): support SEQ_CATEGORY_ID(DENSE(...)) to fill FieldMeta
		FieldMeta: &codegen.FieldMeta{
			Name:     key,
			DType:    codegen.Int,
			IsSparse: false,
		},
		BucketSize: bucketSize,
	}, nil
}

func parseEmbeddingColumn(el *exprlist) (*codegen.EmbeddingColumn, error) {
	help := "EMBEDDING(CATEGORY_ID(...), SIZE[, COMBINER, INITIALIZER])"
	if len(*el) < 4 || len(*el) > 5 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s, should be like: %s", *el, help)
	}
	sourceExprList := (*el)[1]
	if sourceExprList.typ != 0 {
		// key is a IDET string
		// TODO(typhoonzero): support auto add embedding key as a category_id_column
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s, should be like: %s", *el, help)
	}

	// TODO(typhoonzero): user may write EMBEDDING(SPARSE(...)) or EMBEDDING(DENSE(...)),
	// should call parseFieldMeta if error here.
	source, err := parseFeatureColumn(&sourceExprList.sexp)
	if err != nil {
		return nil, err
	}

	// TODO(uuleon) support other kinds of categorical column in the future
	var catColumn interface{}
	catColumn, ok := source.(*columns.CategoryIDColumn)
	if !ok {
		catColumn, ok = source.(*columns.SequenceCategoryIDColumn)
		if !ok {
			return nil, fmt.Errorf("key of EMBEDDING must be categorical column")
		}
	}

	dimension, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad EMBEDDING dimension: %s, err: %s", (*el)[2].val, err)
	}
	combiner, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad EMBEDDING combiner: %s, err: %s", (*el)[3], err)
	}
	initializer := ""
	if len(*el) == 5 {
		initializer, err = expression2string((*el)[4])
		if err != nil {
			return nil, fmt.Errorf("bad EMBEDDING initializer: %s, err: %s", (*el)[4], err)
		}
	}
	return &codegen.EmbeddingColumn{
		CategoryColumn: catColumn,
		Dimension:      dimension,
		Combiner:       combiner,
		Initializer:    initializer}, nil
}

// -------------------------- parse utilities --------------------------------------

func parseShape(e *expr) ([]int, error) {
	var shape []int
	intVal, err := strconv.Atoi(e.val)
	if err != nil {
		list, err := parseExpression(e)
		if err != nil {
			return nil, err
		}
		if list, ok := list.([]interface{}); ok {
			shape, err = transformToIntList(list)
			if err != nil {
				return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", e.val, err)
			}
		} else {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", e.val, err)
		}
	} else {
		shape = append(shape, intVal)
	}
	return shape, nil
}
