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
	"text/template"

	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const entryFile = "entry.py"

// wrapper generates a Python program for submit TensorFlow tasks to PAI.
func wrapper(code, dataSource, modelName, cwd string, numPS, numWrokers int) (string, error) {
	f, err := os.Create(filepath.Join(cwd, entryFile))
	if err != nil {
		return "", fmt.Errorf("Create python code failed")
	}
	f.WriteString(code)
	f.Close()

	var tpl = template.Must(template.New("Submit").Parse(tfWrapperTmplText))
	filler := wrapperFiller{
		DataSource: dataSource,
		ModelName:  modelName,
		EntryFile:  entryFile,
		NumPS:      numPS,
		NumWorkers: numWrokers,
	}
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	fmt.Println(program.String())
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
		numPS = numPSAttr.(int)
		// delete attributes so that tensorflow codegen can run.
		delete(ir.Attributes, "train.num_ps")
	}
	numWorkersAttr, ok := ir.Attributes["train.num_workers"]
	if !ok {
		numWorkers = 1
	} else {
		numWorkers = numWorkersAttr.(int)
		// delete attributes so that tensorflow codegen can run.
		delete(ir.Attributes, "train.num_workers")
	}

	program, err := doTrain(ir, modelName)
	if err != nil {
		fmt.Printf("error run doTrain: %v ", err)
		return "", err
	}
	fmt.Printf("generated train program: %s \n", program)

	return wrapper(program, ir.DataSource, modelName, cwd, numPS, numWorkers)
}

func doTrain(ir *ir.TrainStmt, modelName string) (string, error) {
	code, err := tensorflow.Train(ir)
	if err != nil {
		return "", err
	}
	isKeras, estimatorStr := tensorflow.IsKerasModel(ir.Estimator)
	// append code snippet to save model
	var tpl = template.Must(template.New("SaveModel").Parse(tfSaveModelTmplText))
	filler := saveModelFiller{
		DataSource:   ir.DataSource,
		ModelName:    modelName,
		Save:         "model_save",
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
	program, err := doPredict(ir, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, ir.DataSource, modelName, cwd, 0, 1)
}

func doPredict(ir *ir.PredictStmt, modelName string) (string, error) {
	var tpl = template.Must(template.New("Predict").Parse(tfPredictTmplText))
	filler := predictFiller{
		DataSource:  ir.DataSource,
		ModelName:   modelName,
		Select:      ir.Select,
		ResultTable: ir.ResultTable,
	}
	var code bytes.Buffer
	if err := tpl.Execute(&code, filler); err != nil {
		return "", err
	}
	return code.String(), nil
}
