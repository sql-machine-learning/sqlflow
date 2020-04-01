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

package fluid

import (
	"bytes"
	"fmt"
	"os/exec"

	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/workflow/couler"
)

// Codegen generates Fluid program
type Codegen struct{}

// GenCode generates a Fluid program
func (cg *Codegen) GenCode(programIR []ir.SQLFlowStmt, session *pb.Session) (string, error) {
	r, e := couler.GenFiller(programIR, session)
	if e != nil {
		return "", e
	}

	var program bytes.Buffer
	if e = fluidTemplate.Execute(&program, r); e != nil {
		return "", e
	}
	return program.String(), nil
}

// GenYAML translate Fluid program into YAML
func (cg *Codegen) GenYAML(fluidProgram string) (string, error) {
	cmd := exec.Command("python", "-u")
	cmd.Stdin = bytes.NewBufferString(fluidProgram)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed %s, %v %s", cmd, err, out)
	}
	return string(out), nil
}
