// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package couler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var defaultDockerImage = "sqlflow/sqlflow"

// Run generates Couler program
func Run(programIR ir.SQLProgram, session *pb.Session) (string, error) {
	// TODO(yancey1989): fill session as env
	r := &coulerFiller{
		DataSource: session.DbConnStr,
	}
	for _, sqlIR := range programIR {
		ss := &sqlStatment{}
		switch i := sqlIR.(type) {
		case *ir.StandardSQL:
			ss.IsExtendedSQL = false
			ss.OriginalSQL = string(*sqlIR.(*ir.StandardSQL))
		case *ir.TrainStmt:
			ss.IsExtendedSQL = true
			ss.OriginalSQL = sqlIR.(*ir.TrainStmt).OriginalSQL
		case *ir.PredictStmt:
			ss.IsExtendedSQL = true
			ss.OriginalSQL = sqlIR.(*ir.PredictStmt).OriginalSQL
		case *ir.AnalyzeStmt:
			ss.IsExtendedSQL = true
			ss.OriginalSQL = sqlIR.(*ir.AnalyzeStmt).OriginalSQL
		default:
			return "", fmt.Errorf("uncognized IR type: %v", i)
		}
		// NOTE(yancey1989): does not use ModelImage here since the Predict statment
		// does not contain the ModelImage field in SQL Program IR.
		if os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE") != "" {
			ss.DockerImage = os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE")
		} else {
			ss.DockerImage = defaultDockerImage
		}
		r.SQLStatements = append(r.SQLStatements, ss)
	}
	var program bytes.Buffer
	if err := coulerTemplate.Execute(&program, r); err != nil {
		return "", err
	}
	return program.String(), nil
}

func writeArgoFile(coulerFileName string) (string, error) {
	argoYaml, err := ioutil.TempFile("/tmp", "sqlflow-argo*.yaml")
	if err != nil {
		return "", fmt.Errorf("cannot create temporary Argo YAML file: %v", err)
	}
	defer argoYaml.Close()

	cmd := exec.Command("couler", "run", "--mode", "argo", "--file", coulerFileName)
	cmd.Env = append(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("generate Argo workflow yaml error: %v", err)
	}
	argoYaml.Write(out)

	return argoYaml.Name(), nil
}

func writeCoulerFile(programIR ir.SQLProgram, session *pb.Session) (string, error) {
	program, err := Run(programIR, session)
	if err != nil {
		return "", fmt.Errorf("generate couler program error: %v", err)
	}

	coulerFile, err := ioutil.TempFile("/tmp", "sqlflow-couler*.py")
	if err != nil {
		return "", fmt.Errorf("")
	}
	defer coulerFile.Close()
	if _, err := coulerFile.Write([]byte(program)); err != nil {
		return "", err
	}
	return coulerFile.Name(), nil
}

// RunAndWriteArgoFile generates Argo workflow YAML file
func RunAndWriteArgoFile(programIR ir.SQLProgram, session *pb.Session) (string, error) {
	// 1. call codegen_couler.go to genearte Couler program.
	coulerFileName, err := writeCoulerFile(programIR, session)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(coulerFileName)

	// 2. compile Couler program into Argo YAML.
	argoFileName, err := writeArgoFile(coulerFileName)
	if err != nil {
		return "", err
	}
	return argoFileName, nil
}
