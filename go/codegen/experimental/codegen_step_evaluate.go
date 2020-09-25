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
	"fmt"
	"text/template"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type evaluateStepFiller struct {
	StepIndex      int
	DataSource     string
	OriginalSQL    string
	Select         string
	ResultTable    string
	PredLabelName  string
	Load           string
	AttributesJSON string
	Submitter      string
}

// GenerateEvaluation generates the evaluation code
func GenerateEvaluation(evalStmt *ir.EvaluateStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	labelName := ""
	if nc, ok := evalStmt.Label.(*ir.NumericColumn); ok {
		labelName = nc.FieldDesc.Name
	} else {
		return "", fmt.Errorf("unsupported label type %T", evalStmt.Label)
	}

	attrPyStr, err := ir.MarshalToJSONString(evalStmt.Attributes)
	if err != nil {
		return "", err
	}

	filler := &evaluateStepFiller{
		StepIndex:      stepIndex,
		DataSource:     ds,
		OriginalSQL:    escapeSpecialRunesAndTrimSpace(evalStmt.OriginalSQL),
		Select:         escapeSpecialRunesAndTrimSpace(evalStmt.Select),
		ResultTable:    evalStmt.Into,
		PredLabelName:  labelName,
		Load:           evalStmt.ModelName,
		AttributesJSON: attrPyStr,
		Submitter:      getSubmitter(session),
	}

	var program bytes.Buffer
	tpl := template.Must(template.New("Evaluate").Parse(evaluateStepTemplate))
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

const evaluateStepTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    import runtime.temp_file as temp_file
    from runtime.{{.Submitter}} import evaluate
    attrJsonStr = '''{{.AttributesJSON}}'''
    if attrJsonStr != "":
        model_params = json.loads(attrJsonStr)
    else:
        model_params = {}

    with temp_file.TemporaryDirectory(as_cwd=True):
        evaluate(datasource='''{{.DataSource}}''', 
                 original_sql='''{{.OriginalSQL}}''',
                 select='''{{.Select}}''',
                 pred_label_name='''{{.PredLabelName}}''',
                 model_name='''{{.Load}}''',
                 model_params=model_params,
                 result_table='''{{.ResultTable}}''')
`
