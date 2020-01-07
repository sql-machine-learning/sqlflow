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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const entryFile = "entry.py"

type clusterConfig struct {
	NumPS      int
	NumWorkers int
	PSCPU      int
	PSGPU      int
	WorkerCPU  int
	WorkerGPU  int
}

func formatCkptDir(modelName string) (string, error) {
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
func wrapper(code, dataSource, modelName, cwd, tmpTrainTable, tmpValTable string, cc *clusterConfig) (string, error) {
	f, err := os.Create(filepath.Join(cwd, entryFile))
	if err != nil {
		return "", fmt.Errorf("Create python code failed")
	}
	f.WriteString(code)
	f.Close()

	if err != nil {
		return "", err
	}
	ossCkptDir, err := formatCkptDir(modelName)
	if err != nil {
		return "", err
	}

	var tpl = template.Must(template.New("Submit").Parse(tfWrapperTmplText))
	filler := wrapperFiller{
		clusterConfig:    *cc,
		DataSource:       dataSource,
		ModelName:        modelName,
		EntryFile:        entryFile,
		PAITrainTable:    tmpTrainTable,
		PAIValidateTable: tmpValTable,
		OSSCheckpointDir: ossCkptDir,
	}
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

func getClusterConfig(attrs map[string]interface{}) (*clusterConfig, error) {
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
	return &clusterConfig{
		NumPS:      defaultMap["train.num_ps"],
		NumWorkers: defaultMap["train.num_workers"],
		PSCPU:      defaultMap["train.ps_cpu"],
		PSGPU:      defaultMap["train.ps_gpu"],
		WorkerCPU:  defaultMap["train.worker_cpu"],
		WorkerGPU:  defaultMap["train.worker_gpu"],
	}, nil
}

// Train generates a Python program for train a TensorFlow model.
func Train(ir *ir.TrainStmt, session *pb.Session, modelName, cwd string) (string, error) {
	cc, err := getClusterConfig(ir.Attributes)
	program, err := tfTrainAndSave(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpTrainTable, ir.TmpValidateTable, cc)
}

func tfTrainAndSave(ir *ir.TrainStmt, session *pb.Session, modelName string) (string, error) {
	code, err := tensorflow.Train(ir, session)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	isKeras, estimatorStr := tensorflow.IsKerasModel(ir.Estimator)
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	ckptDir, err := formatCkptDir(ir.Into)
	if err != nil {
		return "", err
	}
	filler := saveModelFiller{
		OSSModelDir:  ckptDir,
		Estimator:    estimatorStr,
		IsKerasModel: isKeras,
	}
	var saveCode bytes.Buffer
	if err = tpl.Execute(&saveCode, filler); err != nil {
		return "", err
	}
	return code + saveCode.String(), nil
}

// Predict generates a Python program to predict a TensorFlow model.
func Predict(ir *ir.PredictStmt, session *pb.Session, modelName, cwd string) (string, error) {
	cc, err := getClusterConfig(ir.Attributes)
	if err != nil {
		return "", err
	}
	program, err := tfLoadAndPredict(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpPredictTable, "", cc)
}

func tfLoadAndPredict(ir *ir.PredictStmt, session *pb.Session, modelName string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir, err := formatCkptDir(modelName)
	if err != nil {
		return "", err
	}
	filler := predictFiller{
		OSSModelDir: ossModelDir,
		DataSource:  session.DbConnStr,
		Select:      ir.Select,
		ResultTable: ir.ResultTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}

// Explain generates a Python program to explain a TensorFlow model.
func Explain(ir *ir.ExplainStmt, session *pb.Session, modelName, cwd string) (string, error) {
	cc, err := getClusterConfig(ir.Attributes)
	if err != nil {
		return "", err
	}
	program, err := tfLoadAndExplain(ir, session, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, session.DbConnStr, modelName, cwd,
		ir.TmpExplainTable, "", cc)
}

func tfLoadAndExplain(ir *ir.ExplainStmt, session *pb.Session, modelName string) (string, error) {
	var tpl = template.Must(template.New("Explain").Parse(tfExplainTmplText))
	ossModelDir, err := formatCkptDir(modelName)
	if err != nil {
		return "", err
	}
	filler := explainFiller{
		OSSModelDir: ossModelDir,
		DataSource:  session.DbConnStr,
		Select:      ir.Select,
		ResultTable: ir.Into,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}
