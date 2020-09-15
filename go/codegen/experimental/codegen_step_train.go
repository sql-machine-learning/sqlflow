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

package experimental

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type trainStepFiller struct {
	StepIndex         int
	OriginalSQL       string
	ModelImage        string
	Estimator         string
	DataSource        string
	Select            string
	ValidationSelect  string
	ModelParamsJSON   string
	TrainParamsJSON   string
	FeatureColumnCode string
	LabelColumnCode   string
	Save              string
	Load              string
	DiskCache         bool
	BatchSize         int
	Epoch             int
	Submitter         string
}

func replaceNewLineRuneAndTrimSpace(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// GenerateTrain returns the step code for training.
func GenerateTrain(trainStmt *ir.TrainStmt, stepIndex int, session *pb.Session) (string, error) {
	var err error
	if err = resolveModelParams(trainStmt); err != nil {
		return "", err
	}
	params := parseAttribute(trainStmt.Attributes)
	diskCache := params["train."]["disk_cache"].(bool)
	delete(params["train."], "disk_cache")

	var batchSize, epoch = -1, 1
	batchSizeAttr, ok := params["train."]["batch_size"]
	if ok {
		batchSize = batchSizeAttr.(int)
		delete(params["train."], "batch_size")
	}
	epochAttr, ok := params["train."]["epoch"]
	if ok {
		epoch = epochAttr.(int)
		delete(params["train."], "epoch")
	}
	if _, ok := params["train."]["num_workers"]; ok {
		delete(params["train."], "num_workers")
	}

	if len(trainStmt.Features) > 1 {
		return "", fmt.Errorf("xgboost only support 0 or 1 feature column set, received %d", len(trainStmt.Features))
	}
	// featureColumnCode is a python map definition code like fc_map = {"feature_columns": [...]}
	featureColumnCode := generateFeatureColumnCode(trainStmt.Features)
	labelColumnCode := trainStmt.Label.GenPythonCode()

	mp, err := json.Marshal(params[""])
	if err != nil {
		return "", err
	}
	tp, err := json.Marshal(params["train."])
	if err != nil {
		return "", err
	}

	dbConnStr, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filler := trainStepFiller{
		StepIndex:         stepIndex,
		OriginalSQL:       replaceNewLineRuneAndTrimSpace(trainStmt.OriginalSQL),
		ModelImage:        trainStmt.ModelImage,
		Estimator:         trainStmt.Estimator,
		DataSource:        dbConnStr,
		Select:            replaceNewLineRuneAndTrimSpace(trainStmt.Select),
		ValidationSelect:  replaceNewLineRuneAndTrimSpace(trainStmt.ValidationSelect),
		ModelParamsJSON:   string(mp),
		TrainParamsJSON:   string(tp),
		FeatureColumnCode: featureColumnCode,
		LabelColumnCode:   labelColumnCode,
		Save:              trainStmt.Into,
		Load:              trainStmt.PreTrainedModel,
		DiskCache:         diskCache,
		BatchSize:         batchSize,
		Epoch:             epoch,
		Submitter:         getSubmitter(session),
	}
	var program bytes.Buffer
	var trainTemplate = template.Must(template.New("Train").Parse(trainStepTemplate))
	err = trainTemplate.Execute(&program, filler)
	if err != nil {
		return "", err
	}
	return program.String(), nil
}

const trainStepTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    import runtime.temp_file as temp_file
    import runtime.feature.column
    import runtime.feature.field_desc
    from runtime.{{.Submitter}} import train

    feature_column_map = {{.FeatureColumnCode}}
    label_column = {{.LabelColumnCode}}

    model_params = json.loads('''{{.ModelParamsJSON}}''')
    train_params = json.loads('''{{.TrainParamsJSON}}''')

    with temp_file.TemporaryDirectory(as_cwd=True):
        train_params["disk_cache"] = "{{.DiskCache}}"=="true"
        train_params["batch_size"] = {{.BatchSize}}
        train_params["epoch"] = {{.Epoch}}

        train(datasource='''{{.DataSource}}''',
              original_sql='''{{.OriginalSQL}}''',
              select='''{{.Select}}''',
              validation_select='''{{.ValidationSelect}}''',
              estimator_string='''{{.Estimator}}''',
              model_image='''{{.ModelImage}}''',
              feature_column_map=feature_column_map,
              label_column=label_column,
              model_params=model_params,
              train_params=train_params,
              save='''{{.Save}}''',
              load='''{{.Load}}''')
`
