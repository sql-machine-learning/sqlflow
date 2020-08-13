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

	pb "sqlflow.org/sqlflow/go/proto"
)

var normalStmtStepTmpl = `
def step_entry_{{.StepIndex}}():
    import runtime
    import runtime.dbapi
    conn = runtime.dbapi.connect("{{.DataSource}}")
    stmt = """{{.Stmt}}"""
    if conn.is_query(stmt):
        rs = conn.query(stmt)
        # write rs to stdout using protobuf table writer
    else:
        success = conn.execute(stmt)
        if not success:
            raise Exception("execute statment error: " % stmt)
`

var normalStmtStepTemplate = template.Must(template.New("NormalStmtStep").Parse(normalStmtStepTmpl))

type normalStmtFiller struct {
	StepIndex  int
	DataSource string
	Stmt       string
}

// GenerateNormalStmtStep generate step Python code to run a normal SQL statement.
func GenerateNormalStmtStep(stmt string, session *pb.Session, stepIndex int) (string, error) {
	filler := &normalStmtFiller{
		StepIndex:  stepIndex,
		DataSource: session.DbConnStr,
		Stmt:       stmt,
	}
	var program bytes.Buffer
	if err := normalStmtStepTemplate.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}
