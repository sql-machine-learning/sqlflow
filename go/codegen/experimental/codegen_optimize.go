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
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
	"text/template"
)

func generateOptimizeCode(stmt *ir.OptimizeStmt, stepIndex int, session *pb.Session) (string, error) {
	ds, err := GeneratePyDbConnStr(session)
	if err != nil {
		return "", err
	}

	constraintJSON, err := json.Marshal(stmt.Constraints)
	if err != nil {
		return "", err
	}

	filler := &optimizeFiller{
		StepIndex:       stepIndex,
		DataSource:      ds,
		Select:          escapeSpecialRunesAndTrimSpace(stmt.Select),
		UserID:          session.UserId,
		Variables:       ir.AttrToPythonValue(stmt.Variables),
		ResultValueName: stmt.ResultValueName,
		VariableType:    stmt.VariableType,
		Objective:       ir.AttrToPythonValue(stmt.Objective.ExpressionTokens),
		Direction:       stmt.Direction,
		ConstraintJSON:  string(constraintJSON),
		Solver:          stmt.Solver,
		ResultTable:     stmt.ResultTable,
		Submitter:       getSubmitter(session),
	}

	tpl := template.Must(template.New("Optimize").Parse(optimizeTemplate))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}

type optimizeFiller struct {
	StepIndex       int
	DataSource      string
	Select          string
	Variables       string
	ResultValueName string
	VariableType    string
	Objective       string
	Direction       string
	ConstraintJSON  string
	Solver          string
	ResultTable     string
	UserID          string
	Submitter       string
}

const optimizeTemplate = `
def step_entry_{{.StepIndex}}():
    import json
    from runtime.{{.Submitter}}.optimize import run_optimize

    run_optimize(datasource='''{{.DataSource}}''',
                 select='''{{.Select}}''',
                 variables={{.Variables}},
                 result_value_name='''{{.ResultValueName}}''',
                 variable_type='''{{.VariableType}}''',
                 objective={{.Objective}},
                 direction='''{{.Direction}}''',
                 constraints=json.loads('''{{.ConstraintJSON}}'''),
                 solver='''{{.Solver}}''',
                 result_table='''{{.ResultTable}}''',
                 user_id='''{{.UserID}}''')
`
