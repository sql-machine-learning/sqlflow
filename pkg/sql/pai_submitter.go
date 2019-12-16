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

package sql

import (
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type paiSubmitter struct{ *defaultSubmitter }

func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	if code, e := pai.Train(cl, cl.Into, s.Cwd); e == nil {
		return s.runCommand(code)
	}
	return e
}

func (s *paiSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	pr, e := parser.ParseOneStatement("maxcompute", cl.OriginalSQL)
	if e != nil {
		return e
	}
	if e = createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}
	code, e := pai.Predict(cl, pr.Extended.Model, s.Cwd)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
func init()                                         { submitterRegistry["pai"] = &paiSubmitter{&defaultSubmitter{}} }
