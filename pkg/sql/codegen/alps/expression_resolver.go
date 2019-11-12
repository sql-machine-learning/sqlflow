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

package alps

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/columns"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
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

type resolvedTrainClauseWithIR struct {
	IsPreMadeModel                bool
	ModelName                     string
	ModelConstructorParams        map[string]interface{}
	BatchSize                     int
	Verbose                       int
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
	ValidationTable               string
	EvalSteps                     int
	EvalStartDelay                int
	EvalThrottle                  int
	EvalCheckpointFilenameForInit string
	FeatureColumns                map[string][]codegen.FeatureColumn
	ColumnSpecs                   map[string][]*columns.ColumnSpec
	EngineParams                  engineSpec
	CustomModule                  *gitLabModule
}



type resolvedPredictClauseWithIR struct {
	ModelName                 string
	OutputTable               string
	ModelConstructorParams    map[string]interface{}
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

func getIntAttrWithIR(attrs map[string]interface{}, key string, defaultValue int) int {
	if p, ok := attrs[key]; ok {
		defer delete(attrs, key)
		if ok {
			return p.(int)
		}
		log.Printf("ignore invalid %s=%s, default is %d", key, p.(string), defaultValue)
	}
	return defaultValue
}

func getBoolAttrWithIR(attrs map[string]interface{}, key string, defaultValue bool, optional bool) bool {
	if p, ok := attrs[key]; ok {
		boolVal := p.(bool)
		if !optional {
			defer delete(attrs, key)
		}
		if ok {
			return boolVal
		}
		if !optional {
			log.Printf("ignore invalid %s=%s, default is %v", key, p, defaultValue)
		}
	}
	return defaultValue
}


func getStringAttrWithIR(attrs map[string]interface{}, key string, defaultValue string) string {
	if p, ok := attrs[key]; ok {
		strVal, _ := p.(string)
		defer delete(attrs, key)
		return trimQuotes(strVal)
	}
	return defaultValue
}


func resolveTrainClauseWithIR(tc *codegen.TrainIR) (*resolvedTrainClauseWithIR, error) {
	// slct := tc.Select
	modelName := tc.Estimator
	preMadeModel := !strings.ContainsAny(modelName, ".")
	attrs := tc.Attributes
	
	engineParams := make(map[string]interface{})
	modelParams := make(map[string]interface{})
	for attrKey, attr := range attrs {
		if strings.HasPrefix(attrKey, "engine.") {
			engineParams[strings.Replace(attrKey, "engine.", "", 1)] = attr
			delete(attrs, attrKey)
		}
		if strings.HasPrefix(attrKey, "model.") {
			modelParams[strings.Replace(attrKey, "model.", "", 1)] = attr
			delete(attrs, attrKey)
		}
	}
	
	// modelParams := attrFilter(attrs, "model", true)
	// engineParams := attrFilter(attrs, "engine", true)
	
	batchSize := getIntAttrWithIR(attrs, "train.batch_size", 1)
	dropRemainder := getBoolAttrWithIR(attrs, "train.drop_remainder", true, false)
	cachePath := ""
	var enableCache bool
	if enableCache = getBoolAttrWithIR(attrs, "train.cache", false, true); !enableCache {
		cachePath = getStringAttrWithIR(attrs, "train.cache", "")
		if cachePath != "" {
			enableCache = true
		}
	}
	epoch := getIntAttrWithIR(attrs, "train.epoch", 1)
	shard := getIntAttrWithIR(attrs, "train.shard", 1)
	verbose := getIntAttrWithIR(attrs, "train.verbose", 0)
	maxSteps := getIntAttrWithIR(attrs, "train.max_steps", -1)

	gradsToWait := getIntAttrWithIR(attrs, "train.grads_to_wait", 2)
	tensorboardLogDir := getStringAttrWithIR(attrs, "train.tensorboard_log_dir", "")
	checkpointSteps := getIntAttrWithIR(attrs, "train.checkpoint_steps", 0)
	checkpointDir := getStringAttrWithIR(attrs, "train.checkpoint_dir", "")
	keepCheckpointMax := getIntAttrWithIR(attrs, "train.keep_checkpoint_max", 0)
	fmt.Printf("ly_debug11 +%v \n",maxSteps)
	var shuffleBufferSize int
	var enableShuffle bool
	if enableShuffle = getBoolAttrWithIR(attrs, "train.shuffle", false, true); !enableShuffle {
		shuffleBufferSize = getIntAttrWithIR(attrs, "train.shuffle", 0)
		if shuffleBufferSize > 0 {
			enableShuffle = true
		}
	} else {
		shuffleBufferSize = 10240
	}

	evalBatchSize := getIntAttrWithIR(attrs, "eval.batch_size", 1)
	evalSteps := getIntAttrWithIR(attrs, "eval.steps", -1)
	evalStartDecaySecs := getIntAttrWithIR(attrs, "eval.start_delay_secs", 120)
	evalThrottleSecs := getIntAttrWithIR(attrs, "eval.throttle_secs", 600)
	evalCheckpointFilenameForInit := getStringAttrWithIR(attrs, "eval.checkpoint_filename_for_init", "")

	customModel := func() *gitLabModule {
		if preMadeModel == false {
			project := getStringAttrWithIR(attrs, "gitlab.project", "")
			sha := getStringAttrWithIR(attrs, "gitlab.sha", "")
			token := getStringAttrWithIR(attrs, "gitlab.token", "")
			server := getStringAttrWithIR(attrs, "gitlab.server", "")
			sourceRoot := getStringAttrWithIR(attrs, "gitlab.source_root", "")
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

	fcMap := map[string][]codegen.FeatureColumn{}
	csMap := map[string][]*columns.ColumnSpec{}
	// todo tc.Label
	for target, columns := range tc.Features {
		fcs, css, err := resolveTrainColumnsWithIR(&columns)
		if err != nil {
			return nil, err
		}
		fcMap[target] = fcs
		csMap[target] = css
	}

	return &resolvedTrainClauseWithIR{
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
		EngineParams:                  getEngineSpecWithIR(engineParams),
		CustomModule:                  customModel,
		Verbose:                       verbose,
	}, nil
}

func resolveTrainColumnsWithIR(columnExprs *[]codegen.FeatureColumn) ([]codegen.FeatureColumn, []*columns.ColumnSpec, error) {
	var fcs = make([]codegen.FeatureColumn, 0)
	var css = make([]*columns.ColumnSpec, 0)
	var cssName = make(map[string]string)
	for _, featureColumn := range *columnExprs {
		fieldMeta := featureColumn.GetFieldMeta()[0]
		if _, ok := cssName[fieldMeta.Name]; !ok {
			cs := &columns.ColumnSpec{
				ColumnName: fieldMeta.Name,
				IsSparse:   fieldMeta.IsSparse,
				Shape:      fieldMeta.Shape,
				DType:      dtypeToString(fieldMeta.DType),
				Delimiter:  fieldMeta.Delimiter}
				css = append(css, cs)
				cssName[fieldMeta.Name] = fieldMeta.Name
		}
		if featureColumn != nil {
			fcs = append(fcs, featureColumn)
		}
	}
	return fcs, css, nil
}

func attrToPythonValue(attr interface{}) string {
	switch attr.(type) {
	case int:
		return fmt.Sprintf("%d", attr.(int))
	case int64:
		return fmt.Sprintf("%d", attr.(int64))
	case float32:
		return fmt.Sprintf("%f", attr.(float32))
	case float64: // FIXME(typhoonzero): may never use
		return fmt.Sprintf("%f", attr.(float64))
	case []int:
		return intArrayToJSONString(attr.([]int))
		// TODO(typhoonzero): support []float etc.
	case []interface{}:
		tmplist := attr.([]interface{})
		if len(tmplist) > 0 {
			if _, ok := tmplist[0].(int); ok {
				intlist := []int{}
				for _, v := range tmplist {
					intlist = append(intlist, v.(int))
				}
				return intArrayToJSONString(intlist)
			}
		}
		// TODO(typhoonzero): support []float etc.
		return "[]"
	case string:
		return attr.(string)
	default:
		return ""
	}
}

func dtypeToString(dt codegen.FieldType) string {
	switch dt {
	case codegen.Float:
		return "float"
	case codegen.Int:
		return "int"
	case codegen.String:
		return "string"
	default:
		return ""
	}
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

