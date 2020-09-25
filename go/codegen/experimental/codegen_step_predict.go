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
	"text/template"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type predStepFiller struct {
	StepIndex      int
	DataSource     string
	OriginalSQL    string
	Select         string
	PredLabelName  string
	ResultTable    string
	ModelParamJSON string
	Load           string
	Submitter      string
}

// GeneratePredict generates the prediction code.
func GeneratePredict(predStmt *ir.PredictStmt, stepIndex int, session *pb.Session) (string, error) {
	dbConnStr, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	modelParams, err := json.Marshal(predStmt.Attributes)
	if err != nil {
		return "", err
	}

	filler := &predStepFiller{
		StepIndex:      stepIndex,
		DataSource:     dbConnStr,
		OriginalSQL:    escapeSpecialRunesAndTrimSpace(predStmt.OriginalSQL),
		Select:         escapeSpecialRunesAndTrimSpace(predStmt.Select),
		PredLabelName:  predStmt.ResultColumn,
		ResultTable:    predStmt.ResultTable,
		ModelParamJSON: string(modelParams),
		Load:           predStmt.Using,
		Submitter:      getSubmitter(session),
	}

	var program bytes.Buffer
	predTmpl := template.Must(template.New("Train").Parse(predStepTemplate))
	err = predTmpl.Execute(&program, filler)
	if err != nil {
		return "", err
	}
	return program.String(), nil
}

const predStepTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    import runtime.temp_file as temp_file
    from runtime.{{.Submitter}} import pred

    model_params = json.loads('''{{.ModelParamJSON}}''')
    with temp_file.TemporaryDirectory(as_cwd=True):
        pred(datasource='''{{.DataSource}}''', 
             original_sql='''{{.OriginalSQL}}''',
             select='''{{.Select}}''', 
             model_name='''{{.Load}}''',
             label_column='''{{.PredLabelName}}''',
             model_params=model_params,
             result_table='''{{.ResultTable}}''')
`
