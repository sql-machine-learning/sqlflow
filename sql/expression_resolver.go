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
	FeatureColumns                map[string][]featureColumn
	ColumnSpecs                   map[string][]*columnSpec
	EngineParams                  engineSpec
	CustomModule                  *gitLabModule
}

type resolvedPredictClause struct {
	ModelName                 string
	OutputTable               string
	ModelConstructorParams    map[string]*attribute
	CheckpointFilenameForInit string
	EngineParams              engineSpec
}

type featureMap struct {
	Table     string
	Partition string
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func resolveTrainClause(tc *trainClause) (*resolvedTrainClause, error) {
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

	customModel := func() *gitLabModule {
		if preMadeModel == false {
			project := getStringAttr("gitlab.project", "")
			sha := getStringAttr("gitlab.sha", "")
			token := getStringAttr("gitlab.token", "")
			server := getStringAttr("gitlab.server", "")
			sourceRoot := getStringAttr("gitlab.source_root", "")
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
		ColumnSpecs:                   csMap,
		EngineParams:                  getEngineSpec(engineParams),
		CustomModule:                  customModel}, nil
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
	getStringAttr := func(key string, defaultValue string) string {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			defer delete(attrs, p.FullName)
			if err == nil {
				return trimQuotes(strVal)
			}
		}
		return defaultValue
	}
	modelParams := filter(attrs, "model", true)
	engineParams := filter(attrs, "engine", true)

	checkpointFilenameForInit := getStringAttr("predict.checkpoint_filename_for_init", "")

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
func resolveTrainColumns(columns *exprlist) ([]featureColumn, []*columnSpec, error) {
	var fcs = make([]featureColumn, 0)
	var css = make([]*columnSpec, 0)
	for _, expr := range *columns {
		if expr.typ != 0 {
			// Column identifier like "COLUMN a1,b1"
			// FIXME(typhoonzero): infer the column spec here.
			c := &numericColumn{
				Key:   expr.val,
				Shape: []int{1},
				Dtype: "float32",
			}
			fcs = append(fcs, c)
		} else {
			result, cs, err := resolveColumn(&expr.sexp)
			if err != nil {
				return nil, nil, err
			}
			if cs != nil {
				css = append(css, cs)
			}
			if result != nil {
				fcs = append(fcs, result)
			}
		}
	}
	return fcs, css, nil
}

func getExpressionFieldName(expr *expr) (string, error) {
	if expr.typ != 0 {
		return expr.val, nil
	}
	fc, _, err := resolveColumn(&expr.sexp)
	if err != nil {
		return "", err
	}
	return fc.GetKey(), nil

	// switch r := result.(type) {
	// case *columnSpec:
	// 	return r.ColumnName, nil
	// case featureColumn:
	// 	return r.GetKey(), nil
	// case string:
	// 	return r, nil
	// default:
	// 	return "", fmt.Errorf("getExpressionFieldName: unrecognized type %T", r)
	// }
}

// resolveLispExpression returns the actual value of the expression:
// IDENT -> string
// [1,2,3] -> []interface{}
// ["a","b","c"] -> []interface{}
// [[1,2,3], "b", "c"] -> []interface{}
func resolveLispExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.typ != 0 {
			return expr.val, nil
		}
		return resolveLispExpression(&expr.sexp)
	}

	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of resolveLispExpression must be `expr` or `exprlist` given %s", e)
	}
	headTyp := (*el)[0].typ
	if headTyp == 0 {
		return resolveLispExpression(&(*el)[0].sexp)
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
					value, err := resolveLispExpression(&expr.sexp)
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
	resolved, err := resolveLispExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); ok {
		// FIXME(typhoonzero): remove leading and trailing quotes if needed.
		return strings.Trim(str, "\""), nil
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
