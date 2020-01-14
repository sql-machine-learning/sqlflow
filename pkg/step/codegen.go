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

package step

import (
	"fmt"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/couler"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

// Codegen represents all kinds of Code Generators,
// such as Tensorflow, XGBoost and Couler e.g.
type Codegen interface {
	SQLProgram(sp ir.SQLProgram, sess *pb.Session) (string, error)
	Train(ts *ir.TrainStmt, sess *pb.Session) (string, error)
	Explain(es *ir.ExplainStmt, sess *pb.Session) (string, error)
	Predict(ps *ir.PredictStmt, sess *pb.Session) (string, error)
}

// CodegenRegistry hold the registeried Codegen implementation
var CodegenRegistry = map[string]Codegen{
	"couler": &CoulerCodegen{},
}

// GetCodegen from the registeried code generators.
func GetCodegen(cgName string) (Codegen, error) {
	codegen, ok := CodegenRegistry[cgName]
	if !ok {
		return nil, fmt.Errorf("unreognized codegen: %s", cgName)
	}
	return codegen, nil
}

// RunFromSQLProgram generates program from a SQL program using the Code generator.
func RunFromSQLProgram(cg Codegen, spIR ir.SQLProgram, sess *pb.Session) (string, error) {
	return cg.SQLProgram(spIR, sess)
}

// RunFromSQLStatement generates program from a SQL statment using the Code generator.
func RunFromSQLStatement(cg Codegen, stmt ir.SQLStatement, sess *pb.Session) (string, error) {
	switch ir := stmt.(type) {
	case *ir.TrainStmt:
		return cg.Train(ir, sess)
	case *ir.PredictStmt:
		return cg.Predict(ir, sess)
	case *ir.ExplainStmt:
		return cg.Explain(ir, sess)
	default:
		return "", fmt.Errorf("unrecognized IR Statment: %v", ir)
	}
}

// CoulerCodegen generates Couler program
type CoulerCodegen struct{}

// SQLProgram generates program from SQL program
func (cg *CoulerCodegen) SQLProgram(sp ir.SQLProgram, sess *pb.Session) (string, error) {
	return couler.Run(sp, sess)
}

// Train generates program from Train statment
func (cg *CoulerCodegen) Train(ts *ir.TrainStmt, sess *pb.Session) (string, error) {
	return "", fmt.Errorf("Couler Codegen does not support TrainStmt")
}

// Predict generates program from Predict statment
func (cg *CoulerCodegen) Predict(ts *ir.PredictStmt, sess *pb.Session) (string, error) {
	return "", fmt.Errorf("Couler Codegen does not support PredictStmt")
}

// Explain generates program from Explain statment
func (cg *CoulerCodegen) Explain(es *ir.ExplainStmt, sess *pb.Session) (string, error) {
	return "", fmt.Errorf("Couler Codegen does not support ExplainStmt")
}
