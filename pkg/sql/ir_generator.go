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
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/ir"
)

func generateTrainStmtWithInferredColumns(slct *extendedSelect, connStr string) (*ir.TrainStmt, error) {
	trainStmt, err := generateTrainStmt(slct, connStr)
	if err != nil {
		return nil, err
	}

	if err := InferFeatureColumns(trainStmt); err != nil {
		return nil, err
	}

	return trainStmt, nil
}

func generateTrainStmt(slct *extendedSelect, connStr string) (*ir.TrainStmt, error) {
	tc := slct.trainClause
	modelURI := tc.estimator
	// get model Docker image name
	modelParts := strings.Split(modelURI, "/")
	modelImageName := strings.Join(modelParts[0:len(modelParts)-1], "/")
	modelName := modelParts[len(modelParts)-1]

	attrList, err := generateAttributeIR(&slct.trainAttrs)
	if err != nil {
		return nil, err
	}

	fcMap := make(map[string][]ir.FeatureColumn)
	for target, columnList := range tc.columns {
		fcList := []ir.FeatureColumn{}
		for _, colExpr := range columnList {
			if colExpr.typ != 0 {
				// column identifier like "COLUMN a1,b1"
				nc := &ir.NumericColumn{
					FieldMeta: &ir.FieldMeta{
						Name:      colExpr.val,
						Shape:     []int{1},
						DType:     ir.Float,
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
	label := &ir.NumericColumn{
		FieldMeta: &ir.FieldMeta{
			Name: tc.label,
		}}

	vslct, _ := parseValidationSelect(attrList)
	if vslct == "" {
		vslct = slct.standardSelect.String()
	}
	return &ir.TrainStmt{
		ExtendedSQL: ir.ExtendedSQL{slct.standardSelect.String(), connStr, attrList, ""},
		// TODO(weiguoz): This is a temporary implement. Specifying the
		// validation dataset by keyword `VALIDATE` is the final solution.
		ValidationSelect: vslct,
		ModelImage:       modelImageName,
		Estimator:        modelName,
		Features:         fcMap,
		Label:            label,
		Into:             slct.save,
	}, nil
}

func generateTrainStmtByModel(slct *extendedSelect, connStr, cwd, modelDir, model string) (*ir.TrainStmt, error) {
	db, err := open(connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	slctWithTrain, err := loadModelMeta(slct, db, cwd, modelDir, model)
	if err != nil {
		return nil, err
	}
	return generateTrainStmtWithInferredColumns(slctWithTrain, connStr)
}

func verifyIRWithTrainStmt(sqlir ir.SQLStatement, db *DB) error {
	var selectStmt string
	var trainStmt *ir.TrainStmt
	switch s := sqlir.(type) {
	case *ir.PredictStmt:
		selectStmt = s.Select
		trainStmt = s.TrainStmt
	case *ir.AnalyzeStmt:
		selectStmt = s.Select
		trainStmt = s.TrainStmt
	default:
		return fmt.Errorf("loadModelMetaUsingIR doesn't support IR of type %T", sqlir)
	}

	trainFields, e := verify(selectStmt, db)
	if e != nil {
		return e
	}
	if trainStmt == nil { // Implies we dont' need to load model
		return nil
	}

	predFields, e := verify(trainStmt.Select, db)
	if e != nil {
		return e
	}

	for _, fc := range trainStmt.Features {
		for _, field := range fc {
			for _, fm := range field.GetFieldMeta() {
				name := fm.Name
				it, ok := predFields.get(name)
				if !ok {
					return fmt.Errorf("predFields doesn't contain column %s", name)
				}
				tt, _ := trainFields.get(name)
				if it != tt {
					return fmt.Errorf("field %s type dismatch %v(pred) vs %v(train)", name, it, tt)
				}
			}
		}
	}

	return nil
}

func generatePredictStmt(slct *extendedSelect, connStr string, modelDir string, getTrainStmtFromModel bool) (*ir.PredictStmt, error) {
	attrMap, err := generateAttributeIR(&slct.predAttrs)
	if err != nil {
		return nil, err
	}

	// cwd is used to extract saved model metas to construct the IR.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(cwd)

	var trainStmt *ir.TrainStmt
	if getTrainStmtFromModel {
		trainStmt, err = generateTrainStmtByModel(slct, connStr, cwd, modelDir, slct.model)
		if err != nil {
			return nil, err
		}
	}

	resultTable, resultCol, err := parseResultTable(slct.into)
	if err != nil {
		return nil, err
	}

	predStmt := &ir.PredictStmt{
		ExtendedSQL:  ir.ExtendedSQL{slct.standardSelect.String(), connStr, attrMap, ""},
		ResultTable:  resultTable,
		ResultColumn: resultCol,
		TrainStmt:    trainStmt,
	}

	if getTrainStmtFromModel {
		// FIXME(tony): change the function signature to use *DB
		db, err := NewDB(connStr)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		if err := verifyIRWithTrainStmt(predStmt, db); err != nil {
			return nil, err
		}
	}

	return predStmt, nil
}

func generateAnalyzeStmt(slct *extendedSelect, connStr, modelDir string, getTrainStmtFromModel bool) (*ir.AnalyzeStmt, error) {
	attrs, err := generateAttributeIR(&slct.explainAttrs)
	if err != nil {
		return nil, err
	}

	// cwd is used to extract saved model metas to construct the IR.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(cwd)

	var trainStmt *ir.TrainStmt
	if getTrainStmtFromModel {
		trainStmt, err = generateTrainStmtByModel(slct, connStr, cwd, modelDir, slct.trainedModel)
		if err != nil {
			return nil, err
		}
	}

	analyzeStmt := &ir.AnalyzeStmt{
		ExtendedSQL: ir.ExtendedSQL{slct.standardSelect.String(), connStr, attrs, ""},
		Explainer:   slct.explainer,
		TrainStmt:   trainStmt,
	}

	if getTrainStmtFromModel {
		// FIXME(tony): change the function signature to use *DB
		db, err := NewDB(connStr)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		if err := verifyIRWithTrainStmt(analyzeStmt, db); err != nil {
			return nil, err
		}
	}

	return analyzeStmt, nil
}

func generateAttributeIR(attrs *attrs) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range *attrs {
		resolvedExpr, err := parseExpression(v)
		if err != nil {
			return nil, err
		}
		if _, ok := ret[k]; ok {
			return nil, fmt.Errorf("duplicate attribute: %v=%v", k, resolvedExpr)
		}
		ret[k] = resolvedExpr
	}
	return ret, nil
}

// -------------------------- expression parsers --------------------------------------

// parseExpression is a recursive function, it returns the actual value
// or struct the expression stands for, e.g.
// 1                 ->  int(1)
// "string"          ->  string("string")
// [1,2,3]           ->  []int{1,2,3}
// NUMERIC(c1)       ->  &ir.NumericColumn{Key: "c1"...}
// [NUMERIC(c1), c2] ->  [&ir.NumericColumn{Key: "c1"...}, string("c2")]
//
// parameter e could be type `*expr` or `*exprlist` for recursive call.
func parseExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.typ != 0 {
			return inferStringValue(expr.val), nil
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

func inferStringValue(expr string) interface{} {
	if ret, err := strconv.Atoi(expr); err == nil {
		return ret
	}
	if retFloat, err := strconv.ParseFloat(expr, 32); err == nil {
		// Note(typhoonzero): always use float32 for attributes, we may never use a float64.
		return float32(retFloat)
	}

	// boolean. We pick the candidates which following the SQL usage from
	// implementation of `strconv.ParseBool(expr)`.
	switch expr {
	case "true", "TRUE", "True":
		return true
	case "false", "FALSE", "False":
		return false
	}

	retString := strings.Trim(expr, "\"")
	return strings.Trim(retString, "'")
}

func parseFeatureColumn(el *exprlist) (ir.FeatureColumn, error) {
	head := (*el)[0].val
	if head == "" {
		return nil, fmt.Errorf("column description expects format like NUMERIC(key) etc, got %v", el)
	}

	switch strings.ToUpper(head) {
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

func parseNumericColumn(el *exprlist) (*ir.NumericColumn, error) {
	help := "NUMERIC([DENSE()|SPARSE()|col_name][, SHAPE])"
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad NUMERIC expression format: %s, should be like: %s", *el, help)
	}
	// 1. NUMERIC(DENSE()/SPARSE()) phrases
	if (*el)[1].typ == 0 {
		fieldMeta, err := parseFieldMeta(&(*el)[1].sexp)
		if err != nil {
			return nil, err
		}
		return &ir.NumericColumn{FieldMeta: fieldMeta}, nil
	}
	// 1. NUMERIC(col_name, ...) phrases
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s, should be like: %s", (*el)[1], err, help)
	}
	shape, err := parseShape((*el)[2])
	if err != nil {
		return nil, err
	}

	return &ir.NumericColumn{
		FieldMeta: &ir.FieldMeta{
			Name:     key,
			DType:    ir.Float, // default use float dtype if no DENSE()/SPARSE() provided
			Shape:    shape,
			IsSparse: false,
		},
	}, nil
}

func parseBucketColumn(el *exprlist) (*ir.BucketColumn, error) {
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
	if _, ok := source.(*ir.NumericColumn); !ok {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
	}
	b, err := parseShape(boundariesExprList)
	if err != nil {
		return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
	}
	return &ir.BucketColumn{
		SourceColumn: source.(*ir.NumericColumn),
		Boundaries:   b}, nil
}

func parseCrossColumn(el *exprlist) (*ir.CrossColumn, error) {
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
	return &ir.CrossColumn{
		Keys:           key.([]interface{}),
		HashBucketSize: bucketSize}, nil
}

func parseCategoryIDColumn(el *exprlist) (*ir.CategoryIDColumn, error) {
	help := "CATEGORY_ID([DENSE()|SPARSE()|col_name], BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	var fieldMeta *ir.FieldMeta
	var err error
	if (*el)[1].typ == 0 {
		// CATEGORY_ID(DENSE()/SPARSE()) phrases
		fieldMeta, err = parseFieldMeta(&(*el)[1].sexp)
		if err != nil {
			return nil, err
		}
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad CATEGORY_ID key: %s, err: %s", (*el)[1], err)
		}
		// generate a default FieldMeta
		// TODO(typhoonzero): update default FieldMeta when doing feature derivation
		fieldMeta = &ir.FieldMeta{
			Name:     key,
			DType:    ir.Int,
			IsSparse: false,
			MaxID:    0,
		}
	}
	// FIXME(typhoonzero): support very large bucket size (int64)
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &ir.CategoryIDColumn{
		FieldMeta:  fieldMeta,
		BucketSize: int64(bucketSize),
	}, nil
}

func parseSeqCategoryIDColumn(el *exprlist) (*ir.SeqCategoryIDColumn, error) {
	help := "SEQ_CATEGORY_ID([DENSE()|SPARSE()|col_name], BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	var fieldMeta *ir.FieldMeta
	var err error
	if (*el)[1].typ == 0 {
		// CATEGORY_ID(DENSE()/SPARSE()) phrases
		fieldMeta, err = parseFieldMeta(&(*el)[1].sexp)
		if err != nil {
			return nil, err
		}
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad SEQ_CATEGORY_ID key: %s, err: %s", (*el)[1], err)
		}
		// generate a default FieldMeta
		// TODO(typhoonzero): update default FieldMeta when doing feature derivation
		fieldMeta = &ir.FieldMeta{
			Name:     key,
			DType:    ir.Int,
			IsSparse: false,
			MaxID:    0,
		}
	}

	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &ir.SeqCategoryIDColumn{
		FieldMeta:  fieldMeta,
		BucketSize: bucketSize,
	}, nil
}

func parseEmbeddingColumn(el *exprlist) (*ir.EmbeddingColumn, error) {
	help := "EMBEDDING([CATEGORY_ID(...)|col_name], SIZE, COMBINER[, INITIALIZER])"
	if len(*el) < 4 || len(*el) > 5 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s, should be like: %s", *el, help)
	}
	var catColumn interface{}
	embColName := "" // only used when catColumn == nil
	sourceExprList := (*el)[1]
	if sourceExprList.typ != 0 {
		// 1. key is a IDET string: EMBEDDING(col_name, size), fill a nil in CategoryColumn for later
		// feature derivation.
		catColumn = nil
		embColName = sourceExprList.val
	} else {
		source, err := parseFeatureColumn(&sourceExprList.sexp)
		if err != nil {
			var tmpCatColumn interface{}
			// 2. source is a FieldMeta like EMBEDDING(SPARSE(...), size)
			fm, err := parseFieldMeta(&sourceExprList.sexp)
			if err != nil {
				return nil, err
			}
			// generate default CategoryIDColumn according to FieldMeta, use shape[0]
			// as category_id_column bucket size.
			if len(fm.Shape) < 1 {
				return nil, fmt.Errorf("invalid FieldMeta Shape: %v", sourceExprList)
			}
			tmpCatColumn = &ir.CategoryIDColumn{
				FieldMeta:  fm,
				BucketSize: int64(fm.Shape[0]),
			}
			catColumn = tmpCatColumn
		} else {
			var tmpCatColumn interface{}
			// 3. source is a FeatureColumn like EMBEDDING(CATEGORY_ID(...), size)
			// TODO(uuleon) support other kinds of categorical column in the future
			tmpCatColumn, ok := source.(*ir.CategoryIDColumn)
			if !ok {
				tmpCatColumn, ok = source.(*ir.SeqCategoryIDColumn)
				if !ok {
					return nil, fmt.Errorf("key of EMBEDDING must be categorical column")
				}
			}
			catColumn = tmpCatColumn
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
	return &ir.EmbeddingColumn{
		CategoryColumn: catColumn,
		Dimension:      dimension,
		Combiner:       combiner,
		Initializer:    initializer,
		Name:           embColName}, nil
}

func parseFieldMeta(el *exprlist) (*ir.FieldMeta, error) {
	help := "DENSE|SPARSE(col_name, SHAPE, DELIMITER[, DTYPE])"
	if len(*el) < 4 {
		return nil, fmt.Errorf("bad FieldMeta format: %s, should be like: %s", *el, help)
	}
	call, err := expression2string((*el)[0])
	if err != nil {
		return nil, fmt.Errorf("bad FieldMeta format: %v, should be like: %s", err, help)
	}
	var isSparse bool
	if strings.ToUpper(call) == "DENSE" {
		isSparse = false
	} else if strings.ToUpper(call) == "SPARSE" {
		isSparse = true
	} else {
		return nil, fmt.Errorf("bad FieldMeta: %s, should be like: %s", call, help)
	}

	name, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad FieldMeta name: %s, err: %s", (*el)[1], err)
	}
	var shape []int
	intShape, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		strShape, err := expression2string((*el)[2])
		if err != nil {
			return nil, fmt.Errorf("bad FieldMeta shape: %s, err: %s", (*el)[2].val, err)
		}
		if strShape != "none" {
			return nil, fmt.Errorf("bad FieldMeta shape: %s, err: %s", (*el)[2].val, err)
		}
	} else {
		shape = append(shape, intShape)
	}
	unresolvedDelimiter, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad FieldMeta delimiter: %s, err: %s", (*el)[1], err)
	}

	delimiter, err := resolveDelimiter(unresolvedDelimiter)
	if err != nil {
		return nil, err
	}

	dtype := ir.Float
	if isSparse {
		dtype = ir.Int
	}
	if len(*el) >= 5 {
		dtypeStr, err := expression2string((*el)[4])
		if err != nil {
			return nil, err
		}
		if dtypeStr == "float" {
			dtype = ir.Float
		} else if dtypeStr == "int" {
			dtype = ir.Int
		} else {
			return nil, fmt.Errorf("bad FieldMeta data type %s", dtypeStr)
		}
	}
	return &ir.FieldMeta{
		Name:      name,
		IsSparse:  isSparse,
		Shape:     shape,
		DType:     dtype,
		Delimiter: delimiter,
		MaxID:     0}, nil
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

func parseAttrsGroup(attrs map[string]interface{}, group string) map[string]interface{} {
	g := make(map[string]interface{})
	for k, v := range attrs {
		if strings.HasPrefix(k, group) {
			subk := strings.SplitN(k, group, 2)
			if len(subk[1]) > 0 {
				g[subk[1]] = v
			}
		}
	}
	return g
}

func parseValidationSelect(attrs map[string]interface{}) (string, error) {
	validation := parseAttrsGroup(attrs, "validation.")
	ds, ok := validation["select"].(string)
	if ok {
		return ds, nil
	}
	return "", fmt.Errorf("validation.select not found")
}

// parseResultTable parse out the table name from the INTO statement
// as the following 3 cases:
// db.table.class_col -> db.table, class_col # cut the column name, using the specified db.
// table.class_col -> table, class_col       # using the default db in DSN.
func parseResultTable(intoStatement string) (string, string, error) {
	resultTableParts := strings.Split(intoStatement, ".")
	if len(resultTableParts) == 3 {
		return strings.Join(resultTableParts[0:2], "."), resultTableParts[2], nil
	} else if len(resultTableParts) == 2 {
		return resultTableParts[0], resultTableParts[1], nil
	} else {
		return "", "", fmt.Errorf("invalid result table format, should be [db.table.class_col] or [table.class_col]")
	}
}
