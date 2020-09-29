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
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/model"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/verifier"
)

const (
	sparse           = "SPARSE"
	cross            = "CROSS"
	categoryID       = "CATEGORY_ID"
	seqCategoryID    = "SEQ_CATEGORY_ID"
	categoryHash     = "CATEGORY_HASH"
	weightedCategory = "WEIGHTED_CATEGORY"
	embedding        = "EMBEDDING"
	indicator        = "INDICATOR"
	bucket           = "BUCKET"
	dense            = "DENSE"
	comma            = "COMMA"
	negative         = "-"
	variables        = "variables"
	variableType     = "var_type"
)

// GenerateTrainStmtWithInferredColumns generates a `TrainStmt` with inferred feature columns
func GenerateTrainStmtWithInferredColumns(slct *parser.SQLFlowSelectStmt, connStr string, modelDir string, cwd string, loadPreTrainedModel bool, verifyLabel bool) (*TrainStmt, error) {
	trainStmt, err := GenerateTrainStmt(slct)
	if err != nil {
		return nil, err
	}

	db, err := database.OpenAndConnectDB(connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := InferFeatureColumns(trainStmt, db); err != nil {
		return nil, err
	}

	err = verifyTrainStmt(trainStmt, db, verifyLabel)
	if err != nil {
		return nil, err
	}

	if loadPreTrainedModel && slct.TrainUsing != "" {
		_, _, err = loadModelMeta(slct, db, cwd, modelDir, slct.TrainUsing)
		if err != nil {
			return nil, err
		}
	}

	return trainStmt, nil
}

// GenerateTrainStmt generates a `TrainStmt` without inferring feature columns
func GenerateTrainStmt(slct *parser.SQLFlowSelectStmt) (*TrainStmt, error) {
	tc := slct.TrainClause
	modelURI := tc.Estimator
	// get model Docker image name
	modelParts := strings.Split(modelURI, "/")
	modelImageName := strings.Join(modelParts[0:len(modelParts)-1], "/")
	modelName := modelParts[len(modelParts)-1]

	attrList, err := generateAttributeIR(&slct.TrainAttrs)
	if err != nil {
		return nil, err
	}

	fcMap := make(map[string][]FeatureColumn)
	for target, columnList := range tc.Columns {
		fcList := []FeatureColumn{}
		for _, colExpr := range columnList {
			if colExpr.Type != 0 {
				// column identifier like "COLUMN a1,b1"
				nc := &NumericColumn{
					FieldDesc: &FieldDesc{
						Name:      colExpr.Value,
						Shape:     []int{1},
						DType:     Float,
						IsSparse:  false,
						Delimiter: "",
					}}
				fcList = append(fcList, nc)
			} else {
				fc, err := parseFeatureColumn(&colExpr.Sexp)
				if err != nil {
					return nil, fmt.Errorf("parse FeatureColumn failed: %v", err)
				}
				fcList = append(fcList, fc)
			}
		}
		fcMap[target] = fcList
	}
	label := &NumericColumn{
		FieldDesc: &FieldDesc{
			Name: tc.Label,
		}}

	vslct, _ := parseValidationSelect(attrList)
	trainStmt := &TrainStmt{
		Select: slct.StandardSelect.String(),
		// TODO(weiguoz): This is a temporary implement. Specifying the
		// validation dataset by keyword `VALIDATE` is the final solution.
		ValidationSelect: vslct,
		ModelImage:       modelImageName,
		Estimator:        modelName,
		Attributes:       attrList,
		Features:         fcMap,
		Label:            label,
		PreTrainedModel:  tc.TrainUsing,
		Into:             slct.Save,
	}
	return trainStmt, nil
}

func loadModelMeta(pr *parser.SQLFlowSelectStmt, db *database.DB, cwd, modelDir, modelName string) (*parser.SQLFlowSelectStmt, *parser.SQLFlowSelectStmt, error) {
	modelURI := modelName
	if modelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", modelDir, modelName)
	}

	m, e := model.Load(modelURI, cwd, db)
	if e != nil {
		return nil, nil, fmt.Errorf("load %v", e)
	}
	// Parse the training SELECT statement used to train
	// the model for the prediction.
	tr, e := parser.ParseStatement(db.DriverName, m.TrainSelect)
	if e != nil {
		return nil, nil, fmt.Errorf("parse: TrainSelect %v raise %v", m.TrainSelect, e)
	}

	if e := verifier.VerifyColumnNameAndType(tr.SQLFlowSelectStmt, pr, db); e != nil {
		return nil, nil, fmt.Errorf("VerifyColumnNameAndType: %v", e)
	}

	return pr, tr.SQLFlowSelectStmt, nil
}

// GenerateTrainStmtByModel generates a `TrainStmt` from a trained model
func GenerateTrainStmtByModel(slct *parser.SQLFlowSelectStmt, connStr, cwd, modelDir, model string) (*TrainStmt, error) {
	db, err := database.OpenDB(connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	_, trainSlct, err := loadModelMeta(slct, db, cwd, modelDir, model)
	if err != nil {
		return nil, err
	}

	slct.TrainClause = trainSlct.TrainClause
	return GenerateTrainStmtWithInferredColumns(trainSlct, connStr, "", "", false, false)
}

func verifyTrainStmt(trainStmt *TrainStmt, db *database.DB, verifyLabel bool) error {
	trainFields, e := verifier.Verify(trainStmt.Select, db)
	if e != nil {
		return e
	}

	for _, fc := range trainStmt.Features {
		for _, field := range fc {
			for _, fm := range field.GetFieldDesc() {
				name := fm.Name
				_, ok := trainFields.Get(name)
				if !ok {
					return fmt.Errorf("feature field does not exist in database: %s", name)
				}
			}
		}
	}
	if verifyLabel {
		labelFieldName := trainStmt.Label.GetFieldDesc()[0].Name
		if labelFieldName == "" {
			// empty labelFieldName means clustering model.
			return nil
		}
		_, ok := trainFields.Get(labelFieldName)
		if !ok {
			return fmt.Errorf("label field does not exist in database: %s", labelFieldName)
		}
	}
	return nil
}

func verifyIRWithTrainStmt(sqlir SQLFlowStmt, db *database.DB) error {
	var selectStmt string
	var trainStmt *TrainStmt
	switch s := sqlir.(type) {
	case *PredictStmt:
		selectStmt = s.Select
		trainStmt = s.TrainStmt
	case *ExplainStmt:
		selectStmt = s.Select
		trainStmt = s.TrainStmt
	case *EvaluateStmt:
		selectStmt = s.Select
		trainStmt = s.TrainStmt
	default:
		return fmt.Errorf("verifyIRWithTrainStmt doesn't support IR of type %T", sqlir)
	}

	trainFields, e := verifier.Verify(selectStmt, db)
	if e != nil {
		return e
	}
	if trainStmt == nil { // Implies we don't need to load model
		return nil
	}

	predFields, e := verifier.Verify(trainStmt.Select, db)
	if e != nil {
		return e
	}

	for _, fc := range trainStmt.Features {
		for _, field := range fc {
			for _, fm := range field.GetFieldDesc() {
				name := fm.Name
				it, ok := predFields.Get(name)
				if !ok {
					return fmt.Errorf("predFields doesn't contain column %s", name)
				}
				tt, _ := trainFields.Get(name)
				if it != tt {
					return fmt.Errorf("field %s type dismatch %v(pred) vs %v(train)", name, it, tt)
				}
			}
		}
	}

	return nil
}

// GeneratePredictStmt generates a `PredictStmt` from the parsed result `slct`
func GeneratePredictStmt(slct *parser.SQLFlowSelectStmt, connStr string, modelDir string, cwd string, getTrainStmtFromModel bool) (*PredictStmt, error) {
	attrMap, err := generateAttributeIR(&slct.PredAttrs)
	if err != nil {
		return nil, err
	}

	var trainStmt *TrainStmt
	if getTrainStmtFromModel {
		trainStmt, err = GenerateTrainStmtByModel(slct, connStr, cwd, modelDir, slct.Model)
		if err != nil {
			return nil, err
		}
	}

	resultTable, resultCol, err := parseResultTable(slct.Into)
	if err != nil {
		return nil, err
	}

	predStmt := &PredictStmt{
		Select:       slct.StandardSelect.String(),
		ResultTable:  resultTable,
		ResultColumn: resultCol,
		Attributes:   attrMap,
		Using:        slct.Model,
		TrainStmt:    trainStmt,
	}

	if getTrainStmtFromModel {
		// FIXME(tony): change the function signature to use *database.DB
		db, err := database.OpenAndConnectDB(connStr)
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

// GenerateExplainStmt generates a `ExplainStmt` from the parsed result `slct`
func GenerateExplainStmt(slct *parser.SQLFlowSelectStmt, connStr, modelDir string, cwd string, getTrainStmtFromModel bool) (*ExplainStmt, error) {
	attrs, err := generateAttributeIR(&slct.ExplainAttrs)
	if err != nil {
		return nil, err
	}

	var trainStmt *TrainStmt
	if getTrainStmtFromModel {
		trainStmt, err = GenerateTrainStmtByModel(slct, connStr, cwd, modelDir, slct.TrainedModel)
		if err != nil {
			return nil, err
		}
	}

	explainStmt := &ExplainStmt{
		Select:     slct.StandardSelect.String(),
		Attributes: attrs,
		Explainer:  slct.Explainer,
		TrainStmt:  trainStmt,
		ModelName:  slct.TrainedModel,
		Into:       slct.ExplainInto,
	}

	if getTrainStmtFromModel {
		// FIXME(tony): change the function signature to use *database.DB
		db, err := database.OpenAndConnectDB(connStr)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		if err := verifyIRWithTrainStmt(explainStmt, db); err != nil {
			return nil, err
		}
	}

	return explainStmt, nil
}

// GenerateEvaluateStmt generates a `EvaluateStmt` from the parsed result `slct`
func GenerateEvaluateStmt(slct *parser.SQLFlowSelectStmt, connStr string, modelDir string, cwd string, getTrainStmtFromModel bool) (*EvaluateStmt, error) {
	attrMap, err := generateAttributeIR(&slct.EvaluateAttrs)
	if err != nil {
		return nil, err
	}

	var trainStmt *TrainStmt
	if getTrainStmtFromModel {
		trainStmt, err = GenerateTrainStmtByModel(slct, connStr, cwd, modelDir, slct.ModelToEvaluate)
		if err != nil {
			return nil, err
		}
	}

	label := &NumericColumn{
		FieldDesc: &FieldDesc{
			Name: slct.EvaluateLabel,
		}}

	evaluateStmt := &EvaluateStmt{
		Select:     slct.StandardSelect.String(),
		Attributes: attrMap,
		ModelName:  slct.ModelToEvaluate,
		Label:      label,
		Into:       slct.EvaluateInto,
		TrainStmt:  trainStmt,
	}

	if getTrainStmtFromModel {
		// FIXME(tony): change the function signature to use *database.DB
		db, err := database.OpenAndConnectDB(connStr)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		if err := verifyIRWithTrainStmt(evaluateStmt, db); err != nil {
			return nil, err
		}
	}

	return evaluateStmt, nil
}

func generateAttributeIR(attrs *parser.Attributes) (map[string]interface{}, error) {
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
// DENSE(c1)       ->  &NumericColumn{Key: "c1"...}
// [DENSE(c1), c2] ->  [&NumericColumn{Key: "c1"...}, string("c2")]
//
// parameter e could be type `*expr` or `*parser.ExprList` for recursive call.
func parseExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*parser.Expr); ok {
		if expr.Type != 0 {
			return inferStringValue(expr.Value), nil
		}
		return parseExpression(&expr.Sexp)
	}
	el, ok := e.(*parser.ExprList)
	if !ok {
		return nil, fmt.Errorf("input of parseExpression must be `expr` or `parser.ExprList` given %s", e)
	}

	headTyp := (*el)[0].Type
	if headTyp == parser.IDENT {
		// expression is a function call format like `DENSE(c1)`
		return parseFeatureColumn(el)
	} else if headTyp == '[' {
		// expression is a list of things
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.Sexp == nil {
					intVal, err := strconv.Atoi(expr.Value)
					// TODO(typhoonzero): support list of float etc.
					if err != nil {
						list = append(list, expr.Value)
					} else {
						list = append(list, intVal)
					}
					continue
				}

				/**
				 * Parse negative integer.
				 * See https://github.com/sql-machine-learning/sqlflow/blob/develop/pkg/parser/extended_syntax_parser.y#L371
				 * for the Lisp S-expression of negative number in details.
				 */
				if len(expr.Sexp) == 2 && (*expr.Sexp[0]).Value == negative {
					intVal, err := strconv.Atoi((*expr.Sexp[1]).Value)
					if err == nil {
						list = append(list, -intVal)
						continue
					}
				}

				value, err := parseExpression(&expr.Sexp)
				if err != nil {
					return nil, err
				}
				list = append(list, value)
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
	switch strings.ToUpper(expr) {
	case "TRUE":
		return true
	case "FALSE":
		return false
	}

	retString := strings.Trim(expr, "\"")
	return strings.Trim(retString, "'")
}

func parseFeatureColumn(el *parser.ExprList) (FeatureColumn, error) {
	head := (*el)[0].Value
	if head == "" {
		return nil, fmt.Errorf("column description expects format like DENSE(key) etc, got %v", el)
	}

	switch strings.ToUpper(head) {
	case dense, sparse:
		return parseNumericColumn(el)
	case bucket:
		return parseBucketColumn(el)
	case cross:
		return parseCrossColumn(el)
	case categoryID:
		return parseCategoryIDColumn(el)
	case seqCategoryID:
		return parseSeqCategoryIDColumn(el)
	case categoryHash:
		return parseCategoryHashColumn(el)
	case embedding:
		return parseEmbeddingColumn(el)
	case indicator:
		return parseIndicatorColumn(el)
	case weightedCategory:
		return parseWeightedCategoryColumn(el)
	default:
		return nil, fmt.Errorf("not supported expr: %s", head)
	}
}

func parseDefaultNumericColumn(el *parser.Expr) (*NumericColumn, error) {
	key, err := expression2string(el)
	if err != nil {
		return nil, err
	}
	return &NumericColumn{
		FieldDesc: &FieldDesc{
			Name:     key,
			DType:    Float,
			Shape:    []int{1},
			IsSparse: false,
		},
	}, nil
}

func parseNumericColumn(el *parser.ExprList) (*NumericColumn, error) {
	fieldDesc, err := parseFieldDesc(el)
	if err != nil {
		return nil, err
	}
	return &NumericColumn{
		FieldDesc: fieldDesc,
	}, nil
}

func parseBucketColumn(el *parser.ExprList) (*BucketColumn, error) {
	help := "BUCKET([DENSE(...)|col_name], BOUNDARIES)"
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad BUCKET expression format: %s, should be like: %s", *el, help)
	}

	sourceExprList := (*el)[1]
	boundariesExprList := (*el)[2]

	var source FeatureColumn
	var err error

	if sourceExprList.Type != 0 {
		source, err = parseDefaultNumericColumn(sourceExprList)
		if err != nil {
			return nil, fmt.Errorf("key of BUCKET must be DENSE or column name, which is %s", sourceExprList.Value)
		}
	} else {
		source, err = parseFeatureColumn(&sourceExprList.Sexp)
		if err != nil {
			return nil, fmt.Errorf("key of BUCKET must be DENSE or column name, which is %s", sourceExprList.Sexp)
		}
		if _, ok := source.(*NumericColumn); !ok {
			return nil, fmt.Errorf("key of BUCKET must be DENSE or column name, which is %s", source)
		}
	}

	b, err := parseShape(boundariesExprList)
	if err != nil {
		return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
	}

	for idx := range b {
		if idx >= 1 && b[idx-1] >= b[idx] {
			return nil, fmt.Errorf("BUCKET boundaries should be in strictly ascending order, but got: %d", b)
		}
	}

	return &BucketColumn{
		SourceColumn: source.(*NumericColumn),
		Boundaries:   b}, nil
}

func parseCrossColumn(el *parser.ExprList) (*CrossColumn, error) {
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
	bucketSize, err := strconv.Atoi((*el)[2].Value)
	if err != nil {
		return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].Value, err)
	}
	return &CrossColumn{
		Keys:           key.([]interface{}),
		HashBucketSize: int64(bucketSize)}, nil
}

func parseCategoryIDColumn(el *parser.ExprList) (*CategoryIDColumn, error) {
	help := "CATEGORY_ID([DENSE()|SPARSE()|col_name], BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	var fieldDesc *FieldDesc
	var err error
	if (*el)[1].Type == 0 {
		// CATEGORY_ID(DENSE()/SPARSE()) phrases
		fieldDesc, err = parseFieldDesc(&(*el)[1].Sexp)
		if err != nil {
			return nil, err
		}
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad CATEGORY_ID key: %s, err: %s", (*el)[1], err)
		}
		// generate a default FieldDesc
		fieldDesc = &FieldDesc{
			Name:     strings.ToLower(key),
			DType:    Int,
			IsSparse: false,
			MaxID:    0,
		}
	}
	// FIXME(typhoonzero): support very large bucket size (int64)
	bucketSize, err := strconv.Atoi((*el)[2].Value)
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].Value, err)
	}
	return &CategoryIDColumn{
		FieldDesc:  fieldDesc,
		BucketSize: int64(bucketSize),
	}, nil
}

func parseWeightedCategoryColumn(el *parser.ExprList) (*WeightedCategoryColumn, error) {
	help := "WEIGHTED_CATEGORY([CATEGORY_ID(...)|CATEGORY_HASH(...)])"
	if len(*el) != 2 {
		return nil, fmt.Errorf("bad WEIGHTED_CATEGORY expression format: %s, should be like: %s", *el, help)
	}
	catColumn, colName, err := buildCategoryIDForEmbeddingOrIndicator(el)
	if err != nil {
		return nil, err
	}
	if catColumn == nil {
		return nil, fmt.Errorf("must use specify category column type when using WEIGHTED_CATEGORY like %s", help)
	}

	return &WeightedCategoryColumn{
		CategoryColumn: catColumn,
		Name:           colName,
	}, nil
}

func parseSeqCategoryIDColumn(el *parser.ExprList) (*SeqCategoryIDColumn, error) {
	help := "SEQ_CATEGORY_ID([DENSE()|SPARSE()|col_name], BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID expression format: %s, should be like: %s", *el, help)
	}
	var fieldDesc *FieldDesc
	var err error
	if (*el)[1].Type == 0 {
		// CATEGORY_ID(DENSE()/SPARSE()) phrases
		fieldDesc, err = parseFieldDesc(&(*el)[1].Sexp)
		if err != nil {
			return nil, err
		}
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad SEQ_CATEGORY_ID key: %s, err: %s", (*el)[1], err)
		}
		// generate a default FieldDesc
		// TODO(typhoonzero): update default FieldDesc when doing feature derivation
		fieldDesc = &FieldDesc{
			Name:     strings.ToLower(key),
			DType:    Int,
			IsSparse: false,
			MaxID:    0,
		}
	}

	bucketSize, err := strconv.Atoi((*el)[2].Value)
	if err != nil {
		return nil, fmt.Errorf("bad SEQ_CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].Value, err)
	}
	return &SeqCategoryIDColumn{
		FieldDesc:  fieldDesc,
		BucketSize: int64(bucketSize),
	}, nil
}

func parseCategoryHashColumn(el *parser.ExprList) (*CategoryHashColumn, error) {
	help := "CATEGORY_HASH([DENSE()|SPARSE()|col_name], BUCKET_SIZE)"
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad CATEGORY_HASH expression format: %s, should be like: %s", *el, help)
	}
	var fieldDesc *FieldDesc
	var err error
	if (*el)[1].Type == 0 {
		// CATEGORY_ID(DENSE()/SPARSE()) phrases
		fieldDesc, err = parseFieldDesc(&(*el)[1].Sexp)
		if err != nil {
			return nil, err
		}
	} else {
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad CATEGORY_HASH key: %s, err: %s", (*el)[1], err)
		}
		// generate a default FieldDesc
		// TODO(typhoonzero): update default FieldDesc when doing feature derivation
		fieldDesc = &FieldDesc{
			Name:     strings.ToLower(key),
			DType:    Int,
			IsSparse: false,
			MaxID:    0,
		}
	}
	bucketSize, err := strconv.Atoi((*el)[2].Value)
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_HASH bucketSize: %s, err: %s", (*el)[2].Value, err)
	}
	return &CategoryHashColumn{
		FieldDesc:  fieldDesc,
		BucketSize: int64(bucketSize),
	}, nil
}

func buildCategoryIDForEmbeddingOrIndicator(el *parser.ExprList) (CategoryColumn, string, error) {
	var catColumn CategoryColumn
	sourceExprList := (*el)[1]
	if sourceExprList.Type != 0 {
		// 1. key is a IDET string: EMBEDDING(col_name, size), fill a nil in CategoryColumn for later
		// feature derivation.
		name, err := expression2string(sourceExprList)
		if err != nil {
			return nil, "", fmt.Errorf("bad INDICATOR/EMBEDDING key: %s, err: %s", sourceExprList, err)
		}
		return nil, name, nil
	}
	source, err := parseFeatureColumn(&sourceExprList.Sexp)
	if err != nil {
		return nil, "", err
	}

	if nc, ok := source.(*NumericColumn); ok {
		// 2. source is a FieldDesc like EMBEDDING(SPARSE(...), size)
		fd := nc.FieldDesc
		// generate default CategoryIDColumn according to FieldDesc, use shape[0]
		// as category_id_column bucket size.
		if len(fd.Shape) < 1 {
			return nil, "", fmt.Errorf("invalid FieldDesc Shape: %v", sourceExprList)
		}
		if fd.DelimiterKV != "" {
			catColumn = &WeightedCategoryColumn{
				CategoryColumn: &CategoryIDColumn{
					FieldDesc:  fd,
					BucketSize: int64(fd.Shape[0]),
				},
				Name: fd.Name,
			}
		} else {
			catColumn = &CategoryIDColumn{
				FieldDesc:  fd,
				BucketSize: int64(fd.Shape[0]),
			}
		}
	} else {
		// 3. source is a FeatureColumn like EMBEDDING(CATEGORY_ID(...), size)
		tmpCatColumn, ok := source.(CategoryColumn)
		if !ok {
			return nil, "", fmt.Errorf("key of EMBEDDING must be categorical column")
		}
		catColumn = tmpCatColumn
	}
	return catColumn, "", nil
}

func parseEmbeddingColumn(el *parser.ExprList) (*EmbeddingColumn, error) {
	help := "EMBEDDING([CATEGORY_ID(...)|col_name], SIZE[, COMBINER, INITIALIZER])"
	if len(*el) < 3 || len(*el) > 5 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s, should be like: %s", *el, help)
	}

	catColumn, colName, err := buildCategoryIDForEmbeddingOrIndicator(el)
	if err != nil {
		return nil, err
	}
	dimension, err := strconv.Atoi((*el)[2].Value)
	if err != nil {
		return nil, fmt.Errorf("bad EMBEDDING dimension: %s, err: %s", (*el)[2].Value, err)
	}

	combiner := "sum"
	if len(*el) >= 4 {
		combiner, err = expression2string((*el)[3])
		if err != nil {
			return nil, fmt.Errorf("bad EMBEDDING combiner: %s, err: %s", (*el)[3], err)
		}
	}

	initializer := ""
	if len(*el) == 5 {
		initializer, err = expression2string((*el)[4])
		if err != nil {
			return nil, fmt.Errorf("bad EMBEDDING initializer: %s, err: %s", (*el)[4], err)
		}
	}
	return &EmbeddingColumn{
		CategoryColumn: catColumn,
		Dimension:      dimension,
		Combiner:       combiner,
		Initializer:    initializer,
		Name:           colName}, nil
}

func parseIndicatorColumn(el *parser.ExprList) (*IndicatorColumn, error) {
	help := "INDICATOR(CATEGORY_ID(...)|CATEGORY_HASH(...)|col_name])"
	if len(*el) < 2 || len(*el) > 3 {
		return nil, fmt.Errorf("bad INDICATOR expression format: %s, should be like: %s", *el, help)
	}
	catColumn, colName, err := buildCategoryIDForEmbeddingOrIndicator(el)
	if err != nil {
		return nil, err
	}
	return &IndicatorColumn{
		CategoryColumn: catColumn,
		Name:           colName}, nil
}

func parseFieldDesc(el *parser.ExprList) (*FieldDesc, error) {
	help := "DENSE|SPARSE(col_name[, SHAPE, DELIMITER, DTYPE, DELIMITER_KV, DTYPE_KEY])"

	if len(*el) < 2 || len(*el) > 7 {
		return nil, fmt.Errorf("bad DENSE|SPARSE format: %v, should be like: %s", *el, help)
	}

	call, err := expression2string((*el)[0])
	if err != nil {
		return nil, fmt.Errorf("bad DENSE|SPARSE format: %v, should be like: %s", err, help)
	}

	var isSparse bool
	head := strings.ToUpper(call)
	if head == dense {
		isSparse = false
	} else if head == sparse {
		isSparse = true
	} else {
		return nil, fmt.Errorf("bad DENSE|SPARSE format: %v, should be like: %s", *el, help)
	}

	help = head + "(col_name[, SHAPE, DELIMITER, DTYPE, DELIMITER_KV, DTYPE_KEY])"

	name, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad %s name: %s, err: %s", head, (*el)[1], err)
	}

	shape := make([]int, 0)
	if len(*el) >= 3 {
		shapeInterface, err := parseExpression((*el)[2])
		if err != nil {
			return nil, fmt.Errorf("bad %s shape: %v, err: %s", head, (*el)[2], err)
		}

		switch s := shapeInterface.(type) {
		case int:
			shape = append(shape, s)
		case []interface{}:
			for _, item := range s {
				if intItem, ok := item.(int); ok {
					shape = append(shape, intItem)
				} else {
					return nil, fmt.Errorf("bad %s shape: %v", head, (*el)[2])
				}
			}
		case string:
			if s != "none" {
				return nil, fmt.Errorf("bad %s shape: %s", head, s)
			}
		default:
			return nil, fmt.Errorf("bad %s shape: %v", head, (*el)[2])
		}
	}

	delimiter := ""
	if len(*el) >= 4 {
		unresolvedDelimiter, err := expression2string((*el)[3])
		if err != nil {
			return nil, fmt.Errorf("bad %s delimiter: %s, err: %s", head, (*el)[1], err)
		}
		delimiter = resolveDelimiter(unresolvedDelimiter)
	}

	dtype := Float
	if len(*el) >= 5 {
		dtypeStr, err := expression2string((*el)[4])
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(dtypeStr, "float") {
			dtype = Float
		} else if strings.EqualFold(dtypeStr, "int") {
			dtype = Int
		} else if strings.EqualFold(dtypeStr, "string") {
			dtype = String
		} else {
			return nil, fmt.Errorf("bad %s data value type %s", head, dtypeStr)
		}
		if len(*el) >= 6 && dtype != Int {
			return nil, fmt.Errorf("when the column of a key-value list format, the key data type must be int, but got %s", dtypeStr)
		}
	}

	// parse delimiter_kv
	delimiterKV := ""
	if len(*el) >= 6 {
		delimiterKV, err = expression2string((*el)[5])
		if err != nil {
			return nil, err
		}
	}
	// parse DTypeWeight
	dtypeWeight := Float
	if len(*el) == 7 {
		dtypeWeightStr, err := expression2string((*el)[6])
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(dtypeWeightStr, "float") {
			dtypeWeight = Float
		} else {
			return nil, fmt.Errorf("bad %s weight type %s, support float for weight only", head, dtypeWeightStr)
		}
	}

	return &FieldDesc{
		Name:        name,
		IsSparse:    isSparse,
		Shape:       shape,
		DType:       dtype,
		Delimiter:   delimiter,
		DelimiterKV: delimiterKV,
		DTypeWeight: dtypeWeight,
	}, nil
}

// -------------------------- parse utilities --------------------------------------

func parseShape(e *parser.Expr) ([]int, error) {
	var shape []int
	intVal, err := strconv.Atoi(e.Value)
	if err != nil {
		list, err := parseExpression(e)
		if err != nil {
			return nil, err
		}
		if list, ok := list.([]interface{}); ok {
			shape, err = transformToIntList(list)
			if err != nil {
				return nil, fmt.Errorf("bad shape: %s, err: %s", e.Value, err)
			}
		} else {
			return nil, fmt.Errorf("bad shape: %s, err: %s", e.Value, err)
		}
	} else {
		shape = append(shape, intVal)
	}
	return shape, nil
}

func parseValidationSelect(attrs map[string]interface{}) (string, error) {
	if attr, ok := attrs["validation.select"]; ok {
		if attrStr, ok := attr.(string); ok {
			return attrStr, nil
		}
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

func resolveDelimiter(delimiter string) string {
	// Keep compatible with previous documents
	// TODO(typhoonzero): support any delimiter string
	if strings.EqualFold(delimiter, comma) {
		return ","
	}
	return delimiter
}

func expression2string(e interface{}) (string, error) {
	if expr, ok := e.(*parser.Expr); ok {
		if expr.Type != 0 {
			if stringValue, ok := inferStringValue(expr.Value).(string); ok {
				return stringValue, nil
			}
		}
	}
	return "", fmt.Errorf("expression expected to be string, actual: %s", e)
}

func transformToIntList(list []interface{}) ([]int, error) {
	var b = make([]int, len(list))
	for idx, item := range list {
		if intVal, ok := item.(int); ok {
			b[idx] = intVal
		} else {
			return nil, fmt.Errorf("type is not int: %s", item)
		}
	}
	return b, nil
}

// GenerateShowTrainStmt a `ShowTrainStmt` from the parsed result `showTrain`
func GenerateShowTrainStmt(showTrain *parser.SQLFlowSelectStmt) (*ShowTrainStmt, error) {
	return &ShowTrainStmt{
		ModelName: showTrain.ShowTrainClause.ModelName,
	}, nil
}

func getOptimizeVariablesAndResultValueName(optimizeStmt *parser.SQLFlowSelectStmt) ([]string, string, error) {
	varsExpr, ok := optimizeStmt.OptimizeAttrs[variables]
	if !ok {
		return nil, "", fmt.Errorf("%s must be provided inside WITH statement of optimize clause", variables)
	}

	parsedVarsExpr, err := parseExpression(varsExpr)
	if err != nil {
		return nil, "", err
	}

	varsStr, ok := parsedVarsExpr.(string)
	if !ok {
		return nil, "", fmt.Errorf("variables must be string but got %T", parsedVarsExpr)
	}

	varsStr = strings.TrimSpace(varsStr)
	resultName := ""
	if idx := strings.Index(varsStr, "("); idx >= 0 {
		if varsStr[len(varsStr)-1] != ')' {
			return nil, "", fmt.Errorf("invalid format of variables attributes %s", varsStr)
		}

		resultName = strings.TrimSpace(varsStr[0:idx])
		varsStr = strings.TrimSpace(varsStr[idx+1 : len(varsStr)-1])
	} else {
		varsStr = strings.TrimSpace(varsStr)
	}

	varList := strings.Split(varsStr, ",")
	for idx, value := range varList {
		varList[idx] = strings.TrimSpace(value)
		if varList[idx] == "" {
			return nil, "", fmt.Errorf("variable name is empty")
		}
	}

	if len(varList) == 1 && resultName == "" {
		resultName = varList[0]
	}

	if len(varList) > 1 {
		if resultName == "" {
			return nil, "", fmt.Errorf("result name must be provided when there are multiple variables")
		}

		for _, eachVar := range varList {
			if eachVar == resultName {
				return nil, "", fmt.Errorf("result name should not have same name with the selected columns")
			}
		}
	}

	return varList, resultName, nil
}

// GenerateOptimizeStmt generates a `OptimizeStmt` from the parsed result `optimizeStmt`
func GenerateOptimizeStmt(optimizeStmt *parser.SQLFlowSelectStmt) (*OptimizeStmt, error) {
	vars, resultValueName, err := getOptimizeVariablesAndResultValueName(optimizeStmt)
	if err != nil {
		return nil, err
	}

	varTypeExpr, ok := optimizeStmt.OptimizeAttrs[variableType]
	if !ok {
		return nil, fmt.Errorf("%s must be provided in optimize clause", variableType)
	}
	varType, err := parseExpression(varTypeExpr)
	if err != nil {
		return nil, err
	}
	varTypeStr, ok := varType.(string)
	if !ok {
		return nil, fmt.Errorf("%s must be string, but got %T", variableType, varType)
	}

	attrs := make(map[string]interface{})
	for k, attr := range optimizeStmt.OptimizeAttrs {
		if k == variables || k == variableType {
			continue
		}

		parsedAttr, err := parseExpression(attr)
		if err != nil {
			return nil, err
		}
		attrs[k] = parsedAttr
	}

	objective := OptimizeExpr{
		ExpressionTokens: optimizeStmt.Objective.ToTokens(),
	}

	constraints := make([]*OptimizeExpr, len(optimizeStmt.Constraints))
	for i, c := range optimizeStmt.Constraints {
		constraints[i] = &OptimizeExpr{
			ExpressionTokens: c.ToTokens(),
			GroupBy:          c.GroupBy,
		}
	}

	solver := optimizeStmt.Solver
	if solver == "" {
		solver = "glpk" // find a better way to set default value
	}

	stmt := &OptimizeStmt{
		Select:          optimizeStmt.StandardSelect.String(),
		Variables:       vars,
		ResultValueName: resultValueName,
		VariableType:    varTypeStr,
		Attributes:      attrs,
		Objective:       objective,
		Direction:       strings.ToLower(optimizeStmt.Direction),
		Constraints:     constraints,
		Solver:          solver,
		ResultTable:     optimizeStmt.OptimizeInto,
	}

	return stmt, nil
}

// GenerateRunStmt generate the RunStmt result from the parsed result of `TO RUN` statement.
func GenerateRunStmt(slct *parser.SQLFlowSelectStmt) (*RunStmt, error) {
	runStmt := &RunStmt{
		Select:     strings.TrimSpace(slct.StandardSelect.String()),
		ImageName:  slct.ImageName,
		Parameters: slct.Parameters,
		Into:       strings.Join(slct.OutputTables, ","),
	}

	return runStmt, nil
}
