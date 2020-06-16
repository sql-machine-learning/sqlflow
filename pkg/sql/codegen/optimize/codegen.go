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
	"io/ioutil"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	tf "sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"strings"
	"text/template"
)

// GenerateOptimizeCode generates optimize codes for execution
func GenerateOptimizeCode(optimStmt *ir.OptimizeStmt, session *pb.Session, cwd string, paiDBName string, paiTableName string) (string, error) {
	resultTable := optimStmt.ResultTable
	if tf.IsPAI() && !strings.Contains(resultTable, ".") {
		resultTable = fmt.Sprintf("%s.%s", paiDBName, resultTable)
	}

	filler := optimizeFiller{
		UserID:          session.UserId,
		DataSource:      session.DbConnStr,
		Select:          optimStmt.Select,
		Variables:       optimStmt.Variables,
		ResultValueName: optimStmt.ResultValueName,
		VariableType:    optimStmt.VariableType,
		Objective:       optimStmt.Objective.Expression,
		Direction:       optimStmt.Direction,
		Constraints:     optimStmt.Constraints,
		Solver:          optimStmt.Solver,
		ResultTable:     resultTable,
		IsPAI:           tf.IsPAI(),
		PAITrainTable:   fmt.Sprintf("%s.%s", paiDBName, paiTableName),
	}

	var runnerProgram bytes.Buffer
	runnerTpl := template.Must(template.New("custom_optimize_runner").Parse(paiOptFlowRunnerText))
	if err := runnerTpl.Execute(&runnerProgram, filler); err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/custom_optimize_runner.py", cwd), []byte(runnerProgram.String()), 0644); err != nil {
		return "", err
	}

	tpl := template.Must(template.New("optimize").Parse(paiOptFlowSubmitText))
	var program bytes.Buffer
	if err := tpl.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}
