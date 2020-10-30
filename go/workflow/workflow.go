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

package workflow

import (
	"io/ioutil"
	"os"
	"os/exec"

	"sqlflow.org/sqlflow/go/codegen/experimental"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/parser"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql"
	"sqlflow.org/sqlflow/go/workflow/couler"
)

// CompileToYAML compiles the sqlProgram to a YAML workflow
func CompileToYAML(sqlProgram string, session *pb.Session, logger *log.Logger) (string, error) {
	var yaml string

	driverName, _, e := database.ParseURL(session.DbConnStr)
	if e != nil {
		return "", e
	}

	stmts, e := parser.Parse(driverName, sqlProgram)
	if e != nil {
		return "", e
	}
	sqls := sql.RewriteStatementsWithHints(stmts, driverName)
	spIRs, e := sql.ResolveSQLProgram(sqls, logger)
	if e != nil {
		return "", e
	}

	// Generate Fluid/Tekton program
	py, e := couler.GenCode(spIRs, session)
	if e != nil {
		return "", e
	}
	// translate Couler program to workflow YAML
	yaml, e = couler.GenYAML(py)
	if e != nil {
		return "", e
	}
	return yaml, nil
}

// CompileToYAMLExperimental compiles sqlProgram to a YAML workflow.
func CompileToYAMLExperimental(sqlProgram string, session *pb.Session) (string, error) {
	py, e := experimental.GenerateCodeCouler(sqlProgram, session)
	if e != nil {
		return "", e
	}
	tmpfile, e := ioutil.TempFile("/tmp", "couler")
	if e != nil {
		return "", e
	}
	defer os.Remove(tmpfile.Name())
	cmd := exec.Command("python", tmpfile.Name())
	cmd.Env = append(os.Environ())
	yamlBytes, e := cmd.CombinedOutput()
	if e != nil {
		return "", e
	}
	return string(yamlBytes), nil
}
