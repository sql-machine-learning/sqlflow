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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"sqlflow.org/sqlflow/go/codegen/experimental"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/parser"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql"
	"sqlflow.org/sqlflow/go/workflow/argo"
	"sqlflow.org/sqlflow/go/workflow/couler"
)

// Codegen generates workflow YAML
type Codegen interface {
	GenCode([]ir.SQLFlowStmt, *pb.Session) (string, error)
	GenYAML(string) (string, error)
}

// Workflow submits workflow task and trace step status
type Workflow interface {
	Submit(string) (string, error)
	Fetch(*pb.FetchRequest) (*pb.FetchResponse, error)
}

// New returns Codegen and Workflow instance
func New(backend string) (Codegen, Workflow, error) {
	if backend == "couler" {
		return &couler.Codegen{}, &argo.Workflow{}, nil
	} else if backend == "experimental" {
		return nil, &argo.Workflow{}, nil
	}
	return nil, nil, fmt.Errorf("the specified backend: %s has not support", backend)
}

// Run compiles a SQL program to IRs and submits workflow YAML to Kubernetes
func Run(backend string, sqlProgram string, session *pb.Session, logger *log.Logger) (string, error) {
	if backend != "couler" {
		return "", fmt.Errorf("only couler backend is supported")
	}

	driverName, _, e := database.ParseURL(session.DbConnStr)
	if e != nil {
		return "", e
	}
	sqlProgram, e = parser.RemoveCommentInSQLStatement(sqlProgram)
	if e != nil {
		return "", e
	}

	var yaml string
	var wf Workflow

	useExperimentalCodegen := os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") == "true"
	if !useExperimentalCodegen {
		stmts, e := parser.Parse(driverName, sqlProgram)
		if e != nil {
			return "", e
		}
		sqls := sql.RewriteStatementsWithHints(stmts, driverName)
		spIRs, e := sql.ResolveSQLProgram(sqls, logger)
		if e != nil {
			return "", e
		}
		// New Codegen and workflow operator instance according to the backend identifier
		var cg Codegen
		cg, wf, e = New(backend)
		if e != nil {
			return "", e
		}
		// Generate Fluid/Tekton program
		py, e := cg.GenCode(spIRs, session)
		if e != nil {
			return "", e
		}
		// translate Couler program to workflow YAML
		yaml, e = cg.GenYAML(py)
		if e != nil {
			return "", e
		}
	} else {
		// FIXME(typhoonzero): refactor this later
		wf = &argo.Workflow{}
		py, e := experimental.GenerateCodeCouler(sqlProgram, session)
		if e != nil {
			return "", e
		}
		tmpfile, e := ioutil.TempFile("/tmp", "couler")
		if e != nil {
			return "", e
		}
		defer os.Remove(tmpfile.Name())
		pyFileName := tmpfile.Name()
		if _, e = tmpfile.Write([]byte(py)); e != nil {
			tmpfile.Close()
		}
		cmdline := bytes.Buffer{}
		fmt.Fprintf(&cmdline, "couler run --mode argo --workflow_name sqlflow ")
		if c := os.Getenv("SQLFLOW_WORKFLOW_CLUSTER_CONFIG"); len(c) > 0 {
			fmt.Fprintf(&cmdline, "--cluster_config %s ", c)
		}
		fmt.Fprintf(&cmdline, "--file %s", pyFileName)
		coulerExec := strings.Split(cmdline.String(), " ")
		cmd := exec.Command(coulerExec[0], coulerExec[1:]...)
		cmd.Env = append(os.Environ())
		yamlBytes, e := cmd.CombinedOutput()
		if e != nil {
			return "", e
		}
		yaml = string(yamlBytes)
	}

	return wf.Submit(yaml)
}
