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

	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const entryFile = "entry.py"

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
func wrapper(code, dataSource, modelName, cwd, tmpTrainTable, tmpValTable string, numPS, numWrokers int) (string, error) {
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
		DataSource:       dataSource,
		ModelName:        modelName,
		EntryFile:        entryFile,
		NumPS:            numPS,
		NumWorkers:       numWrokers,
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

// Train generates a Python program for train a TensorFlow model.
func Train(ir *ir.TrainStmt, modelName, cwd string) (string, error) {
	var numPS int
	var numWorkers int
	numPSAttr, ok := ir.Attributes["train.num_ps"]
	if !ok {
		numPS = 0
	} else {
		numPS, ok = numPSAttr.(int)
		// NOTE(typhoonzero): pai/codegen.go only deal with train.num_ps and train.num_workers
		// calling attribute validator will also validate attributes defined by tensorflow/codegen.go
		// wich may cause "unsupported attribute", so just manually check in here.
		if !ok {
			return "", fmt.Errorf("train.num_ps should be an integer")
		}
		if numPS < 0 {
			return "", fmt.Errorf("train.num_ps should >= 0")
		}
		// delete attributes so that tensorflow codegen can run.
		delete(ir.Attributes, "train.num_ps")
	}
	numWorkersAttr, ok := ir.Attributes["train.num_workers"]
	if !ok {
		numWorkers = 1
	} else {
		numWorkers, ok = numWorkersAttr.(int)
		if !ok {
			return "", fmt.Errorf("train.num_workers should be an integer")
		}
		if numWorkers < 0 {
			return "", fmt.Errorf("train.num_workers should >= 0")
		}
		// delete attributes so that tensorflow codegen can run.
		delete(ir.Attributes, "train.num_workers")
	}
	program, err := tfTrainAndSave(ir, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, ir.DataSource, modelName, cwd,
		ir.TmpTrainTable, ir.TmpValidateTable, numPS, numWorkers)
}

func tfTrainAndSave(ir *ir.TrainStmt, modelName string) (string, error) {
	code, err := tensorflow.Train(ir)
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

// Predict generates a Python program for train a TensorFlow model.
func Predict(ir *ir.PredictStmt, modelName, cwd string) (string, error) {
	program, err := tfLoadAndPredict(ir, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, ir.DataSource, modelName, cwd,
		ir.TmpPredictTable, "", 0, 1)
}

func tfLoadAndPredict(ir *ir.PredictStmt, modelName string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	ossModelDir, err := formatCkptDir(modelName)
	if err != nil {
		return "", err
	}
	filler := predictFiller{
		OSSModelDir: ossModelDir,
		DataSource:  ir.DataSource,
		Select:      ir.Select,
		ResultTable: ir.ResultTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}
