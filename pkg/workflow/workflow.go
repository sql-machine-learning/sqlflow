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
	"fmt"

	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/workflow/argo"
	"sqlflow.org/sqlflow/pkg/workflow/codegen/couler"
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
	if backend == "argo" {
		return &couler.Codegen{}, &argo.Workflow{}, nil
	}
	return nil, nil, fmt.Errorf("the specifiy backend: %s has not support", backend)
}

// Execute translate SQLProgram IR to workflow YAML and submit to Kubernetes.
func Execute(backend string, sqls []ir.SQLFlowStmt, session *pb.Session) (string, error) {
	cg, wf, e := New(backend)
	if e != nil {
		return "", e
	}

	py, e := cg.GenCode(sqls, session)
	if e != nil {
		return "", e
	}

	yaml, e := cg.GenYAML(py)
	if e != nil {
		return "", e
	}

	return wf.Submit(yaml)
}
