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

	"github.com/sql-machine-learning/sqlflow/sql/columns"
)

const (
	sparse        = "SPARSE"
	numeric       = "NUMERIC"
	cross         = "CROSS"
	categoryID    = "CATEGORY_ID"
	seqCategoryID = "SEQ_CATEGORY_ID"
	embedding     = "EMBEDDING"
	bucket        = "BUCKET"
	square        = "SQUARE"
	dense         = "DENSE"
	comma         = "COMMA"
)

type resourceSpec struct {
	Num    int
	Memory int
	Core   int
}

type resolvedTrainClause struct {
	IsPreMadeModel                bool
	ModelName                     string
	ModelConstructorParams        map[string]*attribute
	BatchSize                     int
	EvalBatchSize                 int
	DropRemainder                 bool
	EnableCache                   bool
	CachePath                     string
	Epoch                         int
	Shard                         int
	EnableShuffle                 bool
	ShuffleBufferSize             int
	MaxSteps                      int
	GradsToWait                   int
	TensorboardLogDir             string
	CheckpointSteps               int
	CheckpointDir                 string
	KeepCheckpointMax             int
	EvalSteps                     int
	EvalStartDelay                int
	EvalThrottle                  int
	EvalCheckpointFilenameForInit string
	FeatureColumns                map[string][]columns.FeatureColumn
	EngineParams                  engineSpec
	CustomModule                  *gitLabModule
	FeatureColumnInfered          FeatureColumnMap
}

type resolvedPredictClause struct {
	ModelName                 string
	OutputTable               string
	ModelConstructorParams    map[string]*attribute
	CheckpointFilenameForInit string
	EngineParams              engineSpec
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func getIntAttr(attrs map[string]*attribute, key string, defaultValue int) int {
	if p, ok := attrs[key]; ok {
		strVal, _ := p.Value.(string)
		intVal, err := strconv.Atoi(trimQuotes(strVal))
		defer delete(attrs, p.FullName)
		if err == nil {
			return intVal
		}
		fmt.Printf("ignore invalid %s=%s, default is %d", key, p.Value, defaultValue)
	}
	return defaultValue
}

func getBoolAttr(attrs map[string]*attribute, key string, defaultValue bool, optional bool) bool {
	if p, ok := attrs[key]; ok {
		strVal, _ := p.Value.(string)
		boolVal, err := strconv.ParseBool(trimQuotes(strVal))
		if !optional {
			defer delete(attrs, p.FullName)
		}
		if err == nil {
			return boolVal
		} else if !optional {
			fmt.Printf("ignore invalid %s=%s, default is %v", key, p.Value, defaultValue)
		}
	}
	return defaultValue
}

func getStringAttr(attrs map[string]*attribute, key string, defaultValue string) string {
	if p, ok := attrs[key]; ok {
		strVal, _ := p.Value.(string)
		defer delete(attrs, p.FullName)
		return trimQuotes(strVal)
	}
	return defaultValue
}

func getStringsAttr(attrs map[string]*attribute, key string, defaultValue []string) []string {
	if p, ok := attrs[key]; ok {
		strVal, _ := p.Value.(string)
		defer delete(attrs, p.FullName)
		return strings.Split(trimQuotes(strVal), ",")
	}
	return defaultValue
}

func resolveTrainClause(tc *trainClause, slct *standardSelect, connConfig *connectionConfig) (*resolvedTrainClause, error) {
	modelName := tc.estimator
	preMadeModel := !strings.ContainsAny(modelName, ".")
	attrs, err := resolveAttribute(&tc.trainAttrs)
	if err != nil {
		return nil, err
	}
	err = ValidateAttributes(attrs)
	if err != nil {
		return nil, err
	}
	modelParams := attrFilter(attrs, "model", true)
	engineParams := attrFilter(attrs, "engine", true)

	batchSize := getIntAttr(attrs, "train.batch_size", 512)
	dropRemainder := getBoolAttr(attrs, "train.drop_remainder", true, false)
	cachePath := ""
	var enableCache bool
	if enableCache = getBoolAttr(attrs, "train.cache", false, true); !enableCache {
		cachePath = getStringAttr(attrs, "train.cache", "")
		if cachePath != "" {
			enableCache = true
		}
	}
	epoch := getIntAttr(attrs, "train.epoch", 1)
	shard := getIntAttr(attrs, "train.shard", 1)
	maxSteps := getIntAttr(attrs, "train.max_steps", -1)

	gradsToWait := getIntAttr(attrs, "train.grads_to_wait", 2)
	tensorboardLogDir := getStringAttr(attrs, "train.tensorboard_log_dir", "")
	checkpointSteps := getIntAttr(attrs, "train.checkpoint_steps", 0)
	checkpointDir := getStringAttr(attrs, "train.checkpoint_dir", "")
	keepCheckpointMax := getIntAttr(attrs, "train.keep_checkpoint_max", 0)

	var shuffleBufferSize int
	var enableShuffle bool
	if enableShuffle = getBoolAttr(attrs, "train.shuffle", false, true); !enableShuffle {
		shuffleBufferSize = getIntAttr(attrs, "train.shuffle", 0)
		if shuffleBufferSize > 0 {
			enableShuffle = true
		}
	} else {
		shuffleBufferSize = 10240
	}

	evalBatchSize := getIntAttr(attrs, "eval.batch_size", 1)
	evalSteps := getIntAttr(attrs, "eval.steps", -1)
	evalStartDecaySecs := getIntAttr(attrs, "eval.start_delay_secs", 120)
	evalThrottleSecs := getIntAttr(attrs, "eval.throttle_secs", 600)
	evalCheckpointFilenameForInit := getStringAttr(attrs, "eval.checkpoint_filename_for_init", "")

	customModel := func() *gitLabModule {
		if preMadeModel == false {
			project := getStringAttr(attrs, "gitlab.project", "")
			sha := getStringAttr(attrs, "gitlab.sha", "")
			token := getStringAttr(attrs, "gitlab.token", "")
			server := getStringAttr(attrs, "gitlab.server", "")
			sourceRoot := getStringAttr(attrs, "gitlab.source_root", "")
			if project == "" {
				return nil
			}
			return &gitLabModule{
				ModuleName:   modelName,
				ProjectName:  project,
				Sha:          sha,
				PrivateToken: token,
				GitLabServer: server,
				SourceRoot:   sourceRoot}
		}
		return nil
	}()

	if len(attrs) > 0 {
		return nil, fmt.Errorf("unsupported parameters: %v", attrs)
	}

	fcMap := map[string][]columns.FeatureColumn{}
	csMap := map[string][]*columns.FieldMeta{}
	for target, columns := range tc.columns {
		fcs, css, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		fcMap[target] = fcs
		csMap[target] = css
	}
	// TODO(typhoonzero): use the derivated maps for codegen, skip checking error
	// since it's not used by codegen yet.
	// also, need to clean up what is inside "resolvedTrainClause", keep only
	// fcInfered, csInfered
	fcInfered, csInfered, err := InferFeatureColumns(slct, fcMap, csMap, connConfig)

	return &resolvedTrainClause{
		IsPreMadeModel:                preMadeModel,
		ModelName:                     modelName,
		ModelConstructorParams:        modelParams,
		BatchSize:                     batchSize,
		EvalBatchSize:                 evalBatchSize,
		DropRemainder:                 dropRemainder,
		EnableCache:                   enableCache,
		CachePath:                     cachePath,
		Epoch:                         epoch,
		Shard:                         shard,
		EnableShuffle:                 enableShuffle,
		ShuffleBufferSize:             shuffleBufferSize,
		MaxSteps:                      maxSteps,
		GradsToWait:                   gradsToWait,
		TensorboardLogDir:             tensorboardLogDir,
		CheckpointSteps:               checkpointSteps,
		CheckpointDir:                 checkpointDir,
		KeepCheckpointMax:             keepCheckpointMax,
		EvalSteps:                     evalSteps,
		EvalStartDelay:                evalStartDecaySecs,
		EvalThrottle:                  evalThrottleSecs,
		EvalCheckpointFilenameForInit: evalCheckpointFilenameForInit,
		FeatureColumns:                fcMap,
		EngineParams:                  getEngineSpec(engineParams),
		CustomModule:                  customModel,
		FeatureColumnInfered:          fcInfered,
	}, nil
}

func resolvePredictClause(pc *predictClause) (*resolvedPredictClause, error) {
	attrs, err := resolveAttribute(&pc.predAttrs)
	if err != nil {
		return nil, err
	}
	err = ValidateAttributes(attrs)
	if err != nil {
		return nil, err
	}

	modelParams := attrFilter(attrs, "model", true)
	engineParams := attrFilter(attrs, "engine", true)

	checkpointFilenameForInit := getStringAttr(attrs, "predict.checkpoint_filename_for_init", "")

	if len(attrs) > 0 {
		return nil, fmt.Errorf("unsupported parameters: %v", attrs)
	}

	return &resolvedPredictClause{
		ModelName:                 pc.model,
		OutputTable:               pc.into,
		ModelConstructorParams:    modelParams,
		CheckpointFilenameForInit: checkpointFilenameForInit,
		EngineParams:              getEngineSpec(engineParams)}, nil
}

// resolveTrainColumns resolve columns from SQL statement,
// returns featureColumn list and featureSpecs
func resolveTrainColumns(columnExprs *exprlist) ([]columns.FeatureColumn, []*columns.FieldMeta, error) {
	var fcs = make([]columns.FeatureColumn, 0)
	var css = make([]*columns.FieldMeta, 0)
	for _, expr := range *columnExprs {
		if expr.typ != 0 {
			// Column identifier like "COLUMN a1,b1"
			c := &columns.NumericColumn{
				Key: expr.val,
			}
			fm := &columns.FieldMeta{
				ColumnName: expr.val,
				Shape:      []int{1},
				DType:      "float32",
			}
			c.FieldMetas = append(c.FieldMetas, fm)
			fcs = append(fcs, c)
		} else {
			result, err := resolveColumn(&expr.sexp)
			if err != nil {
				return nil, nil, err
			}
			if fm, ok := result.(*columns.FieldMeta); ok {
				css = append(css, fm)
			}
			if fc, ok := result.(columns.FeatureColumn); ok {
				fcs = append(fcs, fc)
			}
		}
	}
	return fcs, css, nil
}

func getExpressionFieldName(expr *expr) (string, error) {
	if expr.typ != 0 {
		return expr.val, nil
	}
	result, err := resolveColumn(&expr.sexp)
	if err != nil {
		return "", err
	}
	if fc, ok := result.(columns.FeatureColumn); ok {
		return fc.GetKey(), nil
	}
	return "", fmt.Errorf("expression not a feature column")
}

// resolveExpression parse the expression recursively and
// returns the actual value of the expression:
// featureColumns, columnSpecs, error
// e.g.
// column_1 -> "column_1", nil, nil
// [1,2,3,4] -> [1,2,3,4], nil, nil
// [NUMERIC(col1), col2] -> [*numericColumn, "col2"], nil, nil
func resolveExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.typ != 0 {
			return expr.val, nil
		}
		return resolveExpression(&expr.sexp)
	}
	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of resolveExpression must be `expr` or `exprlist` given %s", e)
	}
	headTyp := (*el)[0].typ
	if headTyp == IDENT {
		// Expression is a function call
		return resolveColumn(el)
	} else if headTyp == '[' {
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.sexp == nil {
					intVal, err := strconv.Atoi(expr.val)
					// TODO: support list of float etc.
					if err != nil {
						list = append(list, expr.val)
					} else {
						list = append(list, intVal)
					}
				} else {
					value, err := resolveExpression(&expr.sexp)
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

func expression2string(e interface{}) (string, error) {
	resolved, err := resolveExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); ok {
		// FIXME(typhoonzero): remove leading and trailing quotes if needed.
		return strings.Trim(str, "\""), nil
	}
	return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
}

func resolveDelimiter(delimiter string) (string, error) {
	if strings.EqualFold(delimiter, comma) {
		return ",", nil
	}
	return "", fmt.Errorf("unsolved delimiter: %s", delimiter)
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

func resolveAttribute(attrs *attrs) (map[string]*attribute, error) {
	ret := make(map[string]*attribute)
	for k, v := range *attrs {
		subs := strings.SplitN(k, ".", 2)
		name := subs[len(subs)-1]
		prefix := ""
		if len(subs) == 2 {
			prefix = subs[0]
		}
		r, err := resolveExpression(v)
		if err != nil {
			return nil, err
		}
		a := &attribute{
			FullName: k,
			Prefix:   prefix,
			Name:     name,
			Value:    r}
		ret[a.FullName] = a
	}
	return ret, nil
}

func resolveBucketColumn(el *exprlist) (*columns.BucketColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
	}
	sourceExprList := (*el)[1]
	boundariesExprList := (*el)[2]
	if sourceExprList.typ != 0 {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %v", sourceExprList)
	}
	source, err := resolveColumn(&sourceExprList.sexp)
	if err != nil {
		return nil, err
	}
	if fc, ok := source.(columns.FeatureColumn); !ok {
		if fc.GetColumnType() != columns.ColumnTypeNumeric {
			return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
		}
	}

	boundaries, err := resolveExpression(boundariesExprList)
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
	return &columns.BucketColumn{
		SourceColumn: source.(*columns.NumericColumn),
		Boundaries:   b}, nil
}

func resolveSeqCategoryIDColumn(el *exprlist) (*columns.SequenceCategoryIDColumn, error) {
	key, bucketSize, delimiter, fm, err := parseCategoryIDColumnExpr(el)
	if err != nil {
		return nil, err
	}
	fc := &columns.SequenceCategoryIDColumn{
		Key:        key,
		BucketSize: bucketSize,
		Delimiter:  delimiter,
		// TODO(typhoonzero): support config dtype
		Dtype: "int64"}
	fc.AppendFieldMetas(fm)
	return fc, nil
}

func resolveCategoryIDColumn(el *exprlist) (*columns.CategoryIDColumn, error) {
	key, bucketSize, delimiter, fm, err := parseCategoryIDColumnExpr(el)
	if err != nil {
		return nil, err
	}
	fc := &columns.CategoryIDColumn{
		Key:        key,
		BucketSize: bucketSize,
		Delimiter:  delimiter,
		// TODO(typhoonzero): support config dtype
		Dtype: "int64"}
	fc.AppendFieldMetas(fm)
	return fc, nil
}

func parseCategoryIDColumnExpr(el *exprlist) (string, int, string, *columns.FieldMeta, error) {
	if len(*el) != 3 && len(*el) != 4 {
		return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID expression format: %s", *el)
	}
	var cs *columns.FieldMeta
	key := ""
	var err error
	if (*el)[1].typ == 0 {
		// explist, maybe DENSE/SPARSE expressions
		subExprList := (*el)[1].sexp
		isSparse := subExprList[0].val == sparse
		cs, err = resolveColumnSpec(&subExprList, isSparse)
		if err != nil {
			return "", 0, "", nil, fmt.Errorf("bad CATEGORY_ID expression format: %v", subExprList)
		}
		key = cs.ColumnName
	} else {
		key, err = expression2string((*el)[1])
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

func resolveCrossColumn(el *exprlist) (*columns.CrossColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
	}
	keysExpr := (*el)[1]
	key, err := resolveExpression(keysExpr)
	if err != nil {
		return nil, err
	}
	if _, ok := key.([]interface{}); !ok {
		return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
	}

	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &columns.CrossColumn{
		Keys:           key.([]interface{}),
		HashBucketSize: bucketSize}, nil
}

func resolveEmbeddingColumn(el *exprlist) (*columns.EmbeddingColumn, error) {
	if len(*el) != 4 && len(*el) != 5 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s", *el)
	}

	sourceExprList := (*el)[1]
	var source interface{}
	var err error
	var innerCategoryColumnKey string

	var catColumnResult interface{}
	if sourceExprList.typ == 0 {
		source, err = resolveColumn(&sourceExprList.sexp)
		if err != nil {
			return nil, err
		}
		// user may write EMBEDDING(SPARSE(...)) or EMBEDDING(DENSE(...))
		fm, ok := source.(*columns.FieldMeta)
		if ok {
			innerCategoryColumnKey = fm.ColumnName
			catColumnResult = &columns.CategoryIDColumn{
				Key:        fm.ColumnName,
				BucketSize: fm.Shape[0],
				Delimiter:  fm.Delimiter,
				Dtype:      fm.DType,
			}
			catColumnResult.(*columns.CategoryIDColumn).AppendFieldMetas(fm)
		} else {
			// TODO(uuleon) support other kinds of categorical column in the future
			var catColumn interface{}
			catColumn, ok = source.(*columns.CategoryIDColumn)
			if !ok {
				catColumn, ok = source.(*columns.SequenceCategoryIDColumn)
				if !ok {
					return nil, fmt.Errorf("key of EMBEDDING must be categorical column")
				}
			}
			catColumnResult = catColumn
			innerCategoryColumnKey = source.(columns.FeatureColumn).GetKey()
		}
	} else {
		// generate a default CategoryIDColumn for later feature derivation.
		catColumnResult = nil
		innerCategoryColumnKey = sourceExprList.val
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
	return &columns.EmbeddingColumn{
		Key:            innerCategoryColumnKey,
		CategoryColumn: catColumnResult,
		Dimension:      dimension,
		Combiner:       combiner,
		Initializer:    initializer}, nil
}

func resolveNumericColumn(el *exprlist) (*columns.NumericColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad NUMERIC expression format: %s", *el)
	}
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s", (*el)[1], err)
	}
	var shape []int
	intVal, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		list, err := resolveExpression((*el)[2])
		if err != nil {
			return nil, err
		}
		if list, ok := list.([]interface{}); ok {
			shape, err = transformToIntList(list)
			if err != nil {
				return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
			}
		} else {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
		}
	} else {
		shape = append(shape, intVal)
	}
	return &columns.NumericColumn{
		Key: key}, nil
}

func resolveColumnSpec(el *exprlist, isSparse bool) (*columns.FieldMeta, error) {
	if len(*el) < 4 {
		return nil, fmt.Errorf("bad FeatureSpec expression format: %s", *el)
	}
	name, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec name: %s, err: %s", (*el)[1], err)
	}
	var shape []int
	intShape, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		strShape, err := expression2string((*el)[2])
		if err != nil {
			return nil, fmt.Errorf("bad FeatureSpec shape: %s, err: %s", (*el)[2].val, err)
		}
		if strShape != "none" {
			return nil, fmt.Errorf("bad FeatureSpec shape: %s, err: %s", (*el)[2].val, err)
		}
	} else {
		shape = append(shape, intShape)
	}
	unresolvedDelimiter, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec delimiter: %s, err: %s", (*el)[1], err)
	}

	delimiter, err := resolveDelimiter(unresolvedDelimiter)
	if err != nil {
		return nil, err
	}

	// resolve feature map
	fm := columns.FeatureMap{}
	dtype := "float"
	if isSparse {
		dtype = "int"
	}
	if len(*el) >= 5 {
		dtype, err = expression2string((*el)[4])
	}
	return &columns.FieldMeta{
		ColumnName: name,
		IsSparse:   isSparse,
		Shape:      shape,
		DType:      dtype,
		Delimiter:  delimiter,
		FeatureMap: fm}, nil
}

// resolveColumn returns the acutal feature column typed struct
// as well as the FieldMeta infomation.
func resolveColumn(el *exprlist) (interface{}, error) {
	head := (*el)[0].val
	if head == "" {
		return nil, fmt.Errorf("column description expects format like NUMERIC(key) etc, got %v", el)
	}

	switch strings.ToUpper(head) {
	case dense:
		fm, err := resolveColumnSpec(el, false)
		if err != nil {
			return nil, err
		}
		return fm, err
	case sparse:
		fm, err := resolveColumnSpec(el, true)
		if err != nil {
			return nil, err
		}
		return fm, err
	case numeric:
		// TODO(typhoonzero): support NUMERIC(DENSE(col)) and NUMERIC(SPARSE(col))
		return resolveNumericColumn(el)
	case bucket:
		return resolveBucketColumn(el)
	case cross:
		return resolveCrossColumn(el)
	case categoryID:
		return resolveCategoryIDColumn(el)
	case seqCategoryID:
		return resolveSeqCategoryIDColumn(el)
	case embedding:
		return resolveEmbeddingColumn(el)
	default:
		return nil, fmt.Errorf("not supported expr: %s", head)
	}
}
