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

type explainStepFiller struct {
	StepIndex      int
	DataSource     string
	OriginalSQL    string
	Select         string
	Explainer      string
	AttributesJSON string
	ResultTable    string
	Load           string
	Submitter      string
}

// GenerateExplain generates the explain code
func GenerateExplain(explainStmt *ir.ExplainStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	attrJSON, err := json.Marshal(explainStmt.Attributes)
	if err != nil {
		return "", err
	}

	filler := &explainStepFiller{
		StepIndex:      stepIndex,
		DataSource:     ds,
		OriginalSQL:    escapeSpecialRunesAndTrimSpace(explainStmt.OriginalSQL),
		Select:         escapeSpecialRunesAndTrimSpace(explainStmt.Select),
		Explainer:      explainStmt.Explainer,
		AttributesJSON: string(attrJSON),
		ResultTable:    explainStmt.Into,
		Load:           explainStmt.ModelName,
		Submitter:      getSubmitter(session),
	}

	tpl := template.Must(template.New("Explain").Parse(explainStepTemplate))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

const explainStepTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    import runtime.temp_file as temp_file
    from runtime.{{.Submitter}} import explain

    attr_json_str = '''{{.AttributesJSON}}'''
    if attr_json_str != "":
        model_params = json.loads('''{{.AttributesJSON}}''')
    else:
        model_params = {}
    
    with temp_file.TemporaryDirectory(as_cwd=True):
        explain(datasource='''{{.DataSource}}''', 
                original_sql='''{{.OriginalSQL}}''',
                select='''{{.Select}}''',
                model_name='''{{.Load}}''',
                model_params=model_params,
                explainer='''{{.Explainer}}''',
                result_table='''{{.ResultTable}}''')
`
