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
	"text/template"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

type runStepFilter struct {
	StepIndex  int
	DataSource string
	Select     string
	ImageName  string
	Parameters string
	Into       string
	Submitter  string
}

// GenerateRun returns the step code for TO RUN.
func GenerateRun(runStmt *ir.RunStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	filter := &runStepFilter{
		StepIndex:  stepIndex,
		DataSource: ds,
		Select:     runStmt.Select,
		ImageName:  runStmt.ImageName,
		Parameters: ir.AttrToPythonValue(runStmt.Parameters),
		Into:       runStmt.Into,
		Submitter:  getSubmitter(session),
	}

	var program bytes.Buffer
	var runTemplate = template.Must(template.New("Run").Parse(runStepTemplate))
	err = runTemplate.Execute(&program, filter)
	if err != nil {
		return "", err
	}

	return program.String(), nil
}

const runStepTemplate = `
def step_entry_{{.StepIndex}}():
    from runtime.{{.Submitter}} import run

    run(datasource='''{{.DataSource}}''',
        select='''{{.Select}}''',
        image_name='''{{.ImageName}}''',
        params={{.Parameters}},
        into='''{{.Into}}''')
`
