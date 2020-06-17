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

package optimize

import (
	"bytes"
	"fmt"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"strings"
	"text/template"
)

// GenerateOptFlowOptimizeCode generates optimize codes for execution
// The returned value is (runnerProgramCode, submitProgramCode, error)
func GenerateOptFlowOptimizeCode(optimStmt *ir.OptimizeStmt, session *pb.Session, dbName, tableName, runnerModuleName string, isPai bool) (string, string, error) {
	resultTable := optimStmt.ResultTable
	if !strings.Contains(resultTable, ".") {
		resultTable = fmt.Sprintf("%s.%s", dbName, resultTable)
	}

	filler := optimizeFiller{
		UserID:          session.UserId,
		Variables:       optimStmt.Variables,
		ResultValueName: optimStmt.ResultValueName,
		VariableType:    optimStmt.VariableType,
		Objective:       optimStmt.Objective,
		Direction:       optimStmt.Direction,
		Constraints:     optimStmt.Constraints,
		Solver:          optimStmt.Solver,
		TrainTable:      fmt.Sprintf("%s.%s", dbName, tableName),
		ResultTable:     resultTable,
		IsPAI:           isPai,
		RunnerModule:    runnerModuleName,
	}

	var runnerProgram bytes.Buffer
	runnerTpl := template.Must(template.New(runnerModuleName).Parse(optFlowRunnerText))
	if err := runnerTpl.Execute(&runnerProgram, filler); err != nil {
		return "", "", err
	}

	tpl := template.Must(template.New("optimize").Parse(optFlowSubmitText))
	var submitProgram bytes.Buffer
	if err := tpl.Execute(&submitProgram, filler); err != nil {
		return "", "", err
	}
	return runnerProgram.String(), submitProgram.String(), nil
}
