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

const normalStmtStepTmpl = `
def step_entry_{{.StepIndex}}():
    import runtime
    import runtime.dbapi
    from runtime.dbapi import table_writer

    conn = runtime.dbapi.connect("{{.DataSource}}")
    stmt = """{{.Stmt}}"""
    if conn.is_query(stmt):
        rs = conn.query(stmt)
        tw = table_writer.ProtobufWriter(rs)
        lines = tw.dump_strings()
        for l in lines:
            print(l)
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

// generateNormalStmtStep generate step Python code to run a normal SQL statement.
func generateNormalStmtStep(stmt string, stepIndex int, session *pb.Session) (string, error) {
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
