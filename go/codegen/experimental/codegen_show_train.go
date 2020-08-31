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
	"html/template"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

func generateShowTrainCode(stmt *ir.ShowTrainStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filler := &showTrainFiller{
		StepIndex:  stepIndex,
		DataSource: ds,
		ModelName:  stmt.ModelName,
		Submitter:  getSubmitter(session),
	}

	tpl := template.Must(template.New("ShowTrain").Parse(showTrainStepCodeTemplate))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

type showTrainFiller struct {
	StepIndex  int
	DataSource string
	ModelName  string
	Submitter  string
}

const showTrainStepCodeTemplate = `
def step_entry_{{.StepIndex}}():
    from runtime.{{.Submitter}} import show_train
    show_train(datasource='''{{.DataSource}}''',
               model_name='''{{.ModelName}}''')
`
