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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

type engineSpec struct {
	etype   string
	ps      resourceSpec
	worker  resourceSpec
	cluster string
	queue   string
}

type gitLabModule struct {
	ModuleName   string
	ProjectName  string
	Sha          string
	PrivateToken string
	SourceRoot   string
	GitLabServer string
}

type resolvedTrainClause struct {
	IsPreMadeModel                   bool
	ModelName                        string
	ModelConstructorParams           map[string]*attribute
	BatchSize                        int
	EvalBatchSize                    int
	DropRemainder                    bool
	EnableCache                      bool
	CachePath                        string
	Epoch                            int
	Shard                            int
	EnableShuffle                    bool
	ShuffleBufferSize                int
	MaxSteps                         int
	GradsToWait                      int
	TensorboardLogDir                string
	CheckpointSteps                  int
	CheckpointDir                    string
	KeepCheckpointMax                int
	EvalSteps                        int
	EvalStartDelay                   int
	EvalThrottle                     int
	EvalCheckpointFilenameForInit    string
	PredictCheckpointFilenameForInit string
	FeatureColumns                   map[string][]featureColumn
	ColumnSpecs                      map[string][]*columnSpec
	EngineParams                     engineSpec
	CustomModule                     *gitLabModule
}

// featureColumn is an interface that all types of feature columns and
// attributes (WITH clause) should follow.
// featureColumn is used to generate feature column code.
type featureColumn interface {
	GenerateCode() (string, error)
	// Some feature columns accept input tensors directly, and the data
	// may be a tensor string like: 12,32,4,58,0,0
	GetDelimiter() string
	GetDtype() string
	GetKey() string
	// return input shape json string, like "[2,3]"
	GetInputShape() string
}

type featureMap struct {
	Table     string
	Partition string
}

// featureSpec contains information to generate DENSE/SPARSE code
type columnSpec struct {
	ColumnName     string
	AutoDerivation bool
	IsSparse       bool
	Shape          []int
	DType          string
	Delimiter      string
	FeatureMap     featureMap
}

type attribute struct {
	FullName string
	Prefix   string
	Name     string
	Value    interface{}
}

type numericColumn struct {
	Key       string
	Shape     []int
	Delimiter string
	Dtype     string
}

type bucketColumn struct {
	SourceColumn *numericColumn
	Boundaries   []int
}

// TODO(uuleon) specify the hash_key if needed
type crossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

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

type embeddingColumn struct {
	CategoryColumn interface{}
	Dimension      int
	Combiner       string
}

func getEngineSpec(attrs map[string]*attribute) engineSpec {
	getInt := func(key string, defaultValue int) int {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			intVal, err := strconv.Atoi(strVal)

			if err == nil {
				return intVal
			}
		}
		return defaultValue
	}
	getString := func(key string, defaultValue string) string {
		if p, ok := attrs[key]; ok {
			strVal, ok := p.Value.(string)
			if ok {
				// TODO(joyyoj): use the parser to do those validations.
				if strings.HasPrefix(strVal, "\"") && strings.HasSuffix(strVal, "\"") {
					return strVal[1 : len(strVal)-1]
				}
				return strVal
			}
		}
		return defaultValue
	}

	psNum := getInt("ps_num", 1)
	psMemory := getInt("ps_memory", 2400)
	workerMemory := getInt("worker_memory", 1600)
	workerNum := getInt("worker_num", 2)
	engineType := getString("type", "local")
	if (psNum > 0 || workerNum > 0) && engineType == "local" {
		engineType = "yarn"
	}
	cluster := getString("cluster", "")
	queue := getString("queue", "")
	return engineSpec{
		etype:   engineType,
		ps:      resourceSpec{Num: psNum, Memory: psMemory},
		worker:  resourceSpec{Num: workerNum, Memory: workerMemory},
		cluster: cluster,
		queue:   queue}
}

func resolveTrainClause(tc *trainClause) (*resolvedTrainClause, error) {
	modelName := tc.estimator
	preMadeModel := !strings.ContainsAny(modelName, ".")
	attrs, err := resolveTrainAttribute(&tc.trainAttrs)
	if err != nil {
		return nil, err
	}
	err = ValidateAttributes(attrs)
	if err != nil {
		return nil, err
	}
	trimQuotes := func(s string) string {
		if len(s) >= 2 {
			if s[0] == '"' && s[len(s)-1] == '"' {
				return s[1 : len(s)-1]
			}
		}
		return s
	}
	getIntAttr := func(key string, defaultValue int) int {
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
	getBoolAttr := func(key string, defaultValue bool, optional bool) bool {
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
	getStringAttr := func(key string, defaultValue string) string {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			defer delete(attrs, p.FullName)
			if err == nil {
				return trimQuotes(strVal)
			}
			fmt.Printf("ignore invalid %s=%s, default is %v", key, p.Value, defaultValue)
		}
		return defaultValue
	}
	modelParams := filter(attrs, "model", true)
	engineParams := filter(attrs, "engine", true)

	batchSize := getIntAttr("train.batch_size", 512)
	dropRemainder := getBoolAttr("train.drop_remainder", true, false)
	cachePath := ""
	var enableCache bool
	if enableCache = getBoolAttr("train.cache", false, true); !enableCache {
		cachePath = getStringAttr("train.cache", "")
		if cachePath != "" {
			enableCache = true
		}
	}
	epoch := getIntAttr("train.epoch", 1)
	shard := getIntAttr("train.shard", 1)
	maxSteps := getIntAttr("train.max_steps", -1)

	gradsToWait := getIntAttr("train.grads_to_wait", 2)
	tensorboardLogDir := getStringAttr("train.tensorboard_log_dir", "")
	checkpointSteps := getIntAttr("train.checkpoint_steps", 0)
	checkpointDir := getStringAttr("train.checkpoint_dir", "")
	keepCheckpointMax := getIntAttr("train.keep_checkpoint_max", 0)

	var shuffleBufferSize int
	var enableShuffle bool
	if enableShuffle = getBoolAttr("train.shuffle", false, true); !enableShuffle {
		shuffleBufferSize = getIntAttr("train.shuffle", 0)
		if shuffleBufferSize > 0 {
			enableShuffle = true
		}
	} else {
		shuffleBufferSize = 10240
	}

	evalBatchSize := getIntAttr("eval.batch_size", 1)
	evalSteps := getIntAttr("eval.steps", -1)
	evalStartDecaySecs := getIntAttr("eval.start_delay_secs", 120)
	evalThrottleSecs := getIntAttr("eval.throttle_secs", 600)
	evalCheckpointFilenameForInit := getStringAttr("eval.checkpoint_filename_for_init", "")

	predictCheckpointFilenameForInit := getStringAttr("predict.checkpoint_filename_for_init", "")

	customModel := func() *gitLabModule {
		if preMadeModel == false {
			project := getStringAttr("gitlab_project", "")
			sha := getStringAttr("gitlab_sha", "")
			token := getStringAttr("gitlab_token", "")
			server := getStringAttr("gitlab_server", "")
			sourceRoot := getStringAttr("gitlab_source_root", "")
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

	fcMap := map[string][]featureColumn{}
	csMap := map[string][]*columnSpec{}
	for target, columns := range tc.columns {
		fcs, css, err := resolveTrainColumns(&columns)
		if err != nil {
			return nil, err
		}
		fcMap[target] = fcs
		csMap[target] = css
	}

	return &resolvedTrainClause{
		IsPreMadeModel:                   preMadeModel,
		ModelName:                        modelName,
		ModelConstructorParams:           modelParams,
		BatchSize:                        batchSize,
		EvalBatchSize:                    evalBatchSize,
		DropRemainder:                    dropRemainder,
		EnableCache:                      enableCache,
		CachePath:                        cachePath,
		Epoch:                            epoch,
		Shard:                            shard,
		EnableShuffle:                    enableShuffle,
		ShuffleBufferSize:                shuffleBufferSize,
		MaxSteps:                         maxSteps,
		GradsToWait:                      gradsToWait,
		TensorboardLogDir:                tensorboardLogDir,
		CheckpointSteps:                  checkpointSteps,
		CheckpointDir:                    checkpointDir,
		KeepCheckpointMax:                keepCheckpointMax,
		EvalSteps:                        evalSteps,
		EvalStartDelay:                   evalStartDecaySecs,
		EvalThrottle:                     evalThrottleSecs,
		EvalCheckpointFilenameForInit:    evalCheckpointFilenameForInit,
		PredictCheckpointFilenameForInit: predictCheckpointFilenameForInit,
		FeatureColumns:                   fcMap,
		ColumnSpecs:                      csMap,
		EngineParams:                     getEngineSpec(engineParams),
		CustomModule:                     customModel}, nil
}

// resolveTrainColumns resolve columns from SQL statement,
// returns featureColumn list and featureSpecs
func resolveTrainColumns(columns *exprlist) ([]featureColumn, []*columnSpec, error) {
	var fcs = make([]featureColumn, 0)
	var css = make([]*columnSpec, 0)
	for _, expr := range *columns {
		result, err := resolveExpression(expr)
		if err != nil {
			return nil, nil, err
		}
		if cs, ok := result.(*columnSpec); ok {
			css = append(css, cs)
			continue
		} else if c, ok := result.(featureColumn); ok {
			fcs = append(fcs, c)
		} else if s, ok := result.(string); ok {
			// simple string column, generate default numeric column
			c := &numericColumn{
				Key:   s,
				Shape: []int{1},
				Dtype: "float32",
			}
			fcs = append(fcs, c)
		} else {
			return nil, nil, fmt.Errorf("not recognized type: %s", result)
		}
	}
	return fcs, css, nil
}

func getExpressionFieldName(expr *expr) (string, error) {
	result, err := resolveExpression(expr)
	if err != nil {
		return "", err
	}
	switch r := result.(type) {
	case *columnSpec:
		return r.ColumnName, nil
	case featureColumn:
		return r.GetKey(), nil
	case string:
		return r, nil
	default:
		return "", fmt.Errorf("getExpressionFieldName: unrecognized type %T", r)
	}
}

// resolveExpression resolve a SQLFlow expression to the actual value
// see: sql.y:241 for the definition of expression.
func resolveExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.val != "" {
			return expr.val, nil
		}
		return resolveExpression(&expr.sexp)
	}

	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of resolveExpression must be `expr` or `exprlist` given %s", e)
	}

	head := (*el)[0].val
	if head == "" {
		return resolveExpression(&(*el)[0].sexp)
	}

	switch strings.ToUpper(head) {
	case dense:
		return resolveColumnSpec(el, false)
	case sparse:
		return resolveColumnSpec(el, true)
	case numeric:
		return resolveNumericColumn(el)
	case bucket:
		return resolveBucketColumn(el)
	case cross:
		return resolveCrossColumn(el)
	case categoryID:
		return resolveCategoryIDColumn(el, false)
	case seqCategoryID:
		return resolveCategoryIDColumn(el, true)
	case embedding:
		return resolveEmbeddingColumn(el)
	case square:
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.sexp == nil {
					intVal, err := strconv.Atoi(expr.val)
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
	default:
		return nil, fmt.Errorf("not supported expr: %s", head)
	}
}

func resolveColumnSpec(el *exprlist, isSparse bool) (*columnSpec, error) {
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
	fm := featureMap{}
	dtype := "float"
	if isSparse {
		dtype = "int"
	}
	if len(*el) >= 5 {
		dtype, err = expression2string((*el)[4])
	}
	return &columnSpec{
		ColumnName:     name,
		AutoDerivation: false,
		IsSparse:       isSparse,
		Shape:          shape,
		DType:          dtype,
		Delimiter:      delimiter,
		FeatureMap:     fm}, nil
}

func expression2string(e interface{}) (string, error) {
	resolved, err := resolveExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
}

func (cs *columnSpec) ToString() string {
	if cs.IsSparse {
		shape := strings.Join(strings.Split(fmt.Sprint(cs.Shape), " "), ",")
		if len(cs.Shape) > 1 {
			groupCnt := len(cs.Shape)
			return fmt.Sprintf("GroupedSparseColumn(name=\"%s\", shape=%s, dtype=\"%s\", group=%d, group_separator='\\002')",
				cs.ColumnName, shape, cs.DType, groupCnt)
		}
		return fmt.Sprintf("SparseColumn(name=\"%s\", shape=%s, dtype=\"%s\")", cs.ColumnName, shape, cs.DType)

	}
	return fmt.Sprintf("DenseColumn(name=\"%s\", shape=%s, dtype=\"%s\", separator=\"%s\")",
		cs.ColumnName,
		strings.Join(strings.Split(fmt.Sprint(cs.Shape), " "), ","),
		cs.DType,
		cs.Delimiter)
}

func resolveNumericColumn(el *exprlist) (*numericColumn, error) {
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
	return &numericColumn{
		Key:   key,
		Shape: shape,
		// FIXME(typhoonzero, tony): support config Delimiter and Dtype
		Delimiter: ",",
		Dtype:     "float32"}, nil
}

func resolveBucketColumn(el *exprlist) (*bucketColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
	}
	sourceExprList := (*el)[1]
	boundariesExprList := (*el)[2]
	source, err := resolveExpression(sourceExprList)
	if err != nil {
		return nil, err
	}
	if _, ok := source.(*numericColumn); !ok {
		return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
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
	return &bucketColumn{
		SourceColumn: source.(*numericColumn),
		Boundaries:   b}, nil
}

func resolveCrossColumn(el *exprlist) (*crossColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
	}
	keysExpr := (*el)[1]
	keys, err := resolveExpression(keysExpr)
	if err != nil {
		return nil, err
	}
	if _, ok := keys.([]interface{}); !ok {
		return nil, fmt.Errorf("bad CROSS keys: %s", err)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &crossColumn{
		Keys:           keys.([]interface{}),
		HashBucketSize: bucketSize}, nil
}

func resolveCategoryIDColumn(el *exprlist, isSequence bool) (interface{}, error) {
	if len(*el) != 3 && len(*el) != 4 {
		return nil, fmt.Errorf("bad CATEGORY_ID expression format: %s", *el)
	}
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID key: %s, err: %s", (*el)[1], err)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CATEGORY_ID bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	delimiter := ""
	if len(*el) == 4 {
		delimiter, err = resolveDelimiter((*el)[3].val)
		if err != nil {
			return nil, fmt.Errorf("bad CATEGORY_ID delimiter: %s, %s", (*el)[3].val, err)
		}
	}
	if isSequence {
		return &sequenceCategoryIDColumn{
			Key:        key,
			BucketSize: bucketSize,
			Delimiter:  delimiter,
			// TODO(typhoonzero): support config dtype
			Dtype:      "int64",
			IsSequence: true}, nil
	}
	return &categoryIDColumn{
		Key:        key,
		BucketSize: bucketSize,
		Delimiter:  delimiter,
		// TODO(typhoonzero): support config dtype
		Dtype: "int64"}, nil
}

func resolveEmbeddingColumn(el *exprlist) (*embeddingColumn, error) {
	if len(*el) != 4 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s", *el)
	}
	sourceExprList := (*el)[1]
	source, err := resolveExpression(sourceExprList)
	if err != nil {
		return nil, err
	}
	// TODO(uuleon) support other kinds of categorical column in the future
	var catColumn interface{}
	catColumn, ok := source.(*categoryIDColumn)
	if !ok {
		catColumn, ok = source.(*sequenceCategoryIDColumn)
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
	return &embeddingColumn{
		CategoryColumn: catColumn,
		Dimension:      dimension,
		Combiner:       combiner}, nil
}

func (nc *numericColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)", nc.Key,
		strings.Join(strings.Split(fmt.Sprint(nc.Shape), " "), ",")), nil
}

func (nc *numericColumn) GetDelimiter() string {
	return nc.Delimiter
}

func (nc *numericColumn) GetDtype() string {
	return nc.Dtype
}

func (nc *numericColumn) GetKey() string {
	return nc.Key
}

func (nc *numericColumn) GetInputShape() string {
	jsonBytes, err := json.Marshal(nc.Shape)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
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

func (cc *crossColumn) GenerateCode() (string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	for idx, key := range cc.Keys {
		if c, ok := key.(featureColumn); ok {
			code, err := c.GenerateCode()
			if err != nil {
				return "", err
			}
			keysGenerated[idx] = code
			continue
		}
		if str, ok := key.(string); ok {
			keysGenerated[idx] = fmt.Sprintf("\"%s\"", str)
		} else {
			return "", fmt.Errorf("cross generate code error, key: %s", key)
		}
	}
	return fmt.Sprintf(
		"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
		strings.Join(keysGenerated, ","), cc.HashBucketSize), nil
}

func (cc *crossColumn) GetDelimiter() string {
	return ""
}

func (cc *crossColumn) GetDtype() string {
	return ""
}

func (cc *crossColumn) GetKey() string {
	// NOTE: cross column is a feature on multiple column keys.
	return ""
}

func (cc *crossColumn) GetInputShape() string {
	// NOTE: return empty since crossed column input shape is already determined
	// by the two crossed columns.
	return ""
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

func (ec *embeddingColumn) GenerateCode() (string, error) {
	catColumn, ok := ec.CategoryColumn.(featureColumn)
	if !ok {
		return "", fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
	}
	sourceCode, err := catColumn.GenerateCode()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
		sourceCode, ec.Dimension, ec.Combiner), nil
}

func generateFeatureColumnCode(fcs []featureColumn) (string, error) {
	var codes = make([]string, 0, len(fcs))
	for _, fc := range fcs {
		code, err := fc.GenerateCode()
		if err != nil {
			return "", nil
		}
		codes = append(codes, code)
	}
	return fmt.Sprintf("[%s]", strings.Join(codes, ",")), nil
}

func (ec *embeddingColumn) GetDelimiter() string {
	return ec.CategoryColumn.(featureColumn).GetDelimiter()
}

func (ec *embeddingColumn) GetDtype() string {
	return ec.CategoryColumn.(featureColumn).GetDtype()
}

func (ec *embeddingColumn) GetKey() string {
	return ec.CategoryColumn.(featureColumn).GetKey()
}

func (ec *embeddingColumn) GetInputShape() string {
	return ec.CategoryColumn.(featureColumn).GetInputShape()
}

func resolveTrainAttribute(attrs *attrs) (map[string]*attribute, error) {
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

func resolveDelimiter(delimiter string) (string, error) {
	if strings.EqualFold(delimiter, comma) {
		return ",", nil
	}
	return "", fmt.Errorf("unsolved delimiter: %s", delimiter)
}

func (a *attribute) GenerateCode() (string, error) {
	if val, ok := a.Value.(string); ok {
		// auto convert to int first.
		if _, err := strconv.Atoi(val); err == nil {
			return fmt.Sprintf("%s=%s", a.Name, val), nil
		}
		return fmt.Sprintf("%s=\"%s\"", a.Name, val), nil
	}
	if val, ok := a.Value.([]interface{}); ok {
		intList, err := transformToIntList(val)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", a.Name,
			strings.Join(strings.Split(fmt.Sprint(intList), " "), ",")), nil
	}
	return "", fmt.Errorf("value of attribute must be string or list of int, given %s", a.Value)
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

func filter(attrs map[string]*attribute, prefix string, remove bool) map[string]*attribute {
	ret := make(map[string]*attribute, 0)
	for _, a := range attrs {
		if strings.EqualFold(a.Prefix, prefix) {
			ret[a.Name] = a
			if remove {
				delete(attrs, a.FullName)
			}
		}
	}
	return ret
}
