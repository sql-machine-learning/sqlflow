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
	"errors"
	"io/ioutil"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"
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
	var yaml_str string

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
	py, e := couler.GenCode(spIRs, session, false)
	if e != nil {
		return "", e
	}
	// translate Couler program to workflow YAML
	yaml_str, e = couler.GenYAML(py)
	if e != nil {
		return "", e
	}
	// patch YAML with service account
	obj := make(map[interface{}]interface{})
	e = yaml.Unmarshal([]byte(yaml_str), &obj)
	if e != nil {
		return "", e
	}
	logger.Errorf("before submit, wfns: (%s), sa: (%s)", session.WfNamespace, session.ServiceAccount)

	if session.WfNamespace != "" {
		metadata, ok := obj["metadata"].(map[interface{}]interface{})
		if !ok {
			return "", errors.New("Can not parse workflow metadata")
		}
		metadata["namespace"] = session.WfNamespace
	}

	if session.ServiceAccount != "" {
		spec, ok := obj["spec"].(map[interface{}]interface{})
		if !ok {
			return "", errors.New("Can not parse workflow spec")
		}
		spec["serviceAccountName"] = session.ServiceAccount
	}
	yaml_bytes, e := yaml.Marshal(obj)
	if e != nil {
		return "", e
	}
	yaml_str = string(yaml_bytes)
	logger.Errorf("final yaml: %s", yaml_str)
	return yaml_str, nil
}

// CompileToCoulerSubmitCode compiles the sqlProgram to a couler python code to submit
func CompileToCoulerSubmitCode(sqlProgram string, session *pb.Session, logger *log.Logger) (string, error) {
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
	py, e := couler.GenCode(spIRs, session, true)
	if e != nil {
		return "", e
	}
	return py, nil
}

// CompileToYAMLExperimental compiles sqlProgram to a YAML workflow.
func CompileToYAMLExperimental(sqlProgram string, session *pb.Session) (string, error) {
	py, e := experimental.GenerateCodeCouler(sqlProgram, session)
	if e != nil {
		return "", e
	}
	tmpfile, e := ioutil.TempFile("/tmp", "sqlflow.py")
	if e != nil {
		return "", e
	}
	defer os.Remove(tmpfile.Name())
	if _, e = tmpfile.Write([]byte(py)); e != nil {
		tmpfile.Close()
	}
	cmd := exec.Command("python", tmpfile.Name())
	cmd.Env = append(os.Environ())
	yamlBytes, e := cmd.CombinedOutput()
	if e != nil {
		return "", e
	}
	return string(yamlBytes), nil
}
