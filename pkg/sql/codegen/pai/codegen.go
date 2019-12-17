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
	"regexp"
	"strings"
	"text/template"

	"sqlflow.org/gomaxcompute"

	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const entryFile = "entry.py"

func getTableFromSelect(dataSource, trainSelect string) (string, string, error) {
	// FIXME(typhoonzero): copied from tensorflow/codegen.go, should remove this and use the temp table
	// in the workflow
	fromRegex, err := regexp.Compile("FROM[\\s\\n]+([\\w\\.]*)")
	if err != nil {
		return "", "", err
	}
	matches := fromRegex.FindAllStringSubmatch(trainSelect, -1)
	if len(matches) != 1 {
		return "", "", fmt.Errorf("only support simple SQL query, but got %s", trainSelect)
	}
	paiTableFull := matches[0][1]
	paiDatabase := ""
	paiTable := ""
	tableParts := strings.Split(paiTableFull, ".")
	if len(tableParts) == 2 {
		paiDatabase = tableParts[0]
		paiTable = tableParts[1]
	} else {
		parts := strings.Split(dataSource, "://")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("error datasource format: %s", dataSource)
		}
		conf, err := gomaxcompute.ParseDSN(parts[1])
		if err != nil {
			return "", "", err
		}
		paiDatabase = conf.Project
		paiTable = paiTableFull
	}
	return paiDatabase, paiTable, nil
}

// wrapper generates a Python program for submit TensorFlow tasks to PAI.
func wrapper(code, dataSource, modelName, cwd string, trainSelect string) (string, error) {
	f, err := os.Create(filepath.Join(cwd, entryFile))
	if err != nil {
		return "", fmt.Errorf("Create python code failed")
	}
	f.WriteString(code)
	f.Close()
	paiDatabase, paiTable, err := getTableFromSelect(dataSource, trainSelect)
	if err != nil {
		return "", err
	}

	var tpl = template.Must(template.New("Submit").Parse(tfWrapperTmplText))
	filler := wrapperFiller{
		DataSource:  dataSource,
		ModelName:   modelName,
		EntryFile:   entryFile,
		PAIDatabase: paiDatabase,
		PAITable:    paiTable,
	}
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

// Train generates a Python program for train a TensorFlow model.
func Train(ir *ir.TrainStmt, modelName, cwd string) (string, error) {
	program, err := doTrain(ir, modelName)
	if err != nil {
		return "", err
	}
	return wrapper(program, ir.DataSource, modelName, cwd, ir.Select)
}

func doTrain(ir *ir.TrainStmt, modelName string) (string, error) {
	code, err := tensorflow.Train(ir)
	if err != nil {
		return "", err
	}

	// append code snippet to save model
	isKeras, estimatorStr := tensorflow.IsKerasModel(ir.Estimator)
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
	return wrapper(program, ir.DataSource, modelName, cwd, ir.Select)
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
