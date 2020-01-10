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

package pai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const entryFile = "entry.py"

// PSConfig implicates Prameter Server Config
type PSConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// WorkerConfig implicates Worker Config
type WorkerConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// ClusterConfig implicits PAI distributed task meta
type ClusterConfig struct {
	PS     PSConfig     `json:"ps"`
	Worker WorkerConfig `json:"worker"`
}

// FormatCkptDir returns the saved model path on OSS
func FormatCkptDir(modelName string) (string, error) {
	ossCkptDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	if ossCkptDir == "" {
		return "", fmt.Errorf("must specify SQLFLOW_OSS_CHECKPOINT_DIR when training with PAI, e.g. oss://bucket/?role_arn=xxx&host=xxx")
	}
	ossURIParts := strings.Split(ossCkptDir, "?") // ossCkptDir: oss://bucket/your/path/?args=...
	if len(ossURIParts) != 2 {
		return "", fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR must be of format: oss://bucket/?role_arn=xxx&host=xxx")
	}
	ossDir := strings.Join([]string{strings.TrimRight(ossURIParts[0], "/"), modelName}, "/")
	// Form URI like: oss://bucket/your/path/modelname/?args=...
	ossCkptDir = strings.Join([]string{ossDir + "/", ossURIParts[1]}, "?")
	return ossCkptDir, nil
}

// wrapper generates a Python program for submit TensorFlow tasks to PAI.
func wrapper(code, dataSource, modelName, cwd, tmpTrainTable, tmpValTable string, resultTable string, cc *ClusterConfig) (string, error) {
	f, err := os.Create(filepath.Join(cwd, entryFile))
	if err != nil {
		return "", fmt.Errorf("Create python code failed")
	}
	f.WriteString(code)
	f.Close()

	if err != nil {
		return "", err
	}
	ossCkptDir, err := FormatCkptDir(modelName)
	if err != nil {
		return "", err
	}

	var tpl = template.Must(template.New("Submit").Parse(tfWrapperTmplText))
	cfString, err := json.Marshal(cc)
	if err != nil {
		return "", err
	}
	isDistributed := false
	if cc.Worker.Count > 1 {
		isDistributed = true
	}
	filler := wrapperFiller{
		ClusterConfigJSON: strconv.Quote(string(cfString)),
		IsDistributed:     isDistributed,
		DataSource:        dataSource,
		ModelName:         modelName,
		EntryFile:         entryFile,
		PAITrainTable:     tmpTrainTable,
		PAIValidateTable:  tmpValTable,
		ResultTable:       resultTable,
		OSSCheckpointDir:  ossCkptDir,
	}
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// GetClusterConfig returns ClusterConfig object comes from WITH clause
func GetClusterConfig(attrs map[string]interface{}) (*ClusterConfig, error) {
	defaultMap := map[string]int{
		"train.num_ps":      0,
		"train.num_workers": 1,
		"train.worker_cpu":  400,
		"train.worker_gpu":  0,
		"train.ps_cpu":      200,
		"train.ps_gpu":      0,
	}
	for k := range defaultMap {
		attrValue, ok := attrs[k]
		if ok {
			intValue, intok := attrValue.(int)
			if !intok {
				return nil, fmt.Errorf("attribute %s must be int, got: %s", k, attrValue)
			}
			defaultMap[k] = intValue
			delete(attrs, k)
		}
	}
	return &ClusterConfig{
		PS: PSConfig{
			Count: defaultMap["train.num_ps"],
			CPU:   defaultMap["train.ps_cpu"],
			GPU:   defaultMap["train.ps_gpu"],
		},
		Worker: WorkerConfig{
			Count: defaultMap["train.num_workers"],
			CPU:   defaultMap["train.worker_cpu"],
			GPU:   defaultMap["train.worker_gpu"],
		},
	}, nil
}

// Train generates a Python program for train a TensorFlow model.
func Train(ir *ir.TrainStmt, session *pb.Session, modelName, cwd string) (string, error) {
	cc, err := GetClusterConfig(ir.Attributes)
	program, err := TFTrainAndSave(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpTrainTable, ir.TmpValidateTable, "", cc)
}

// TFTrainAndSave generates PAI-TF code
func TFTrainAndSave(ir *ir.TrainStmt, session *pb.Session, modelName string) (string, error) {
	code, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	ckptDir, err := FormatCkptDir(ir.Into)
	if err != nil {
		return "", err
	}
	filler := saveModelFiller{
		OSSModelDir: ckptDir,
		Estimator:   ir.Estimator,
	}
	var saveCode bytes.Buffer
	if err = tpl.Execute(&saveCode, filler); err != nil {
		return "", err
	}
	return code + saveCode.String(), nil
}

// Predict generates a Python program for train a TensorFlow model.
func Predict(ir *ir.PredictStmt, session *pb.Session, modelName, cwd string) (string, error) {
	cc, err := GetClusterConfig(ir.Attributes)
	if err != nil {
		return "", err
	}
	program, err := tfLoadAndPredict(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpPredictTable, "", ir.ResultTable, cc)
}

func tfLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelName string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir, err := FormatCkptDir(modelName)
	if err != nil {
		return "", err
	}
	isPAI := (os.Getenv("SQLFLOW_submitter") == "pai" || os.Getenv("SQLFLOW_submitter") == "alisa")
	paiPredictTable := ""
	if isPAI && ir.TmpPredictTable != "" {
		paiPredictTable = ir.TmpPredictTable
	}
	filler := predictFiller{
		OSSModelDir: ossModelDir,
		DataSource:  session.DbConnStr,
		Select:      ir.Select,
		ResultTable: ir.ResultTable,
		IsPAI:       isPAI,
		PAITable:    paiPredictTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}
