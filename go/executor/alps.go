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

package executor

import (
	"fmt"

	"sqlflow.org/sqlflow/go/codegen/alps"
	"sqlflow.org/sqlflow/go/ir"
)

type alpsExecutor struct{ *pythonExecutor }

func (s *alpsExecutor) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return e
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	// TODO(typhoonzero): support using pretrained model for ALPS.

	code, e := alps.Train(cl, s.Session)
	if e != nil {
		return e
	}
	return s.runProgram(code, false)
}

func (s *alpsExecutor) ExecutePredict(stmt *ir.PredictStmt) (e error) {
	return fmt.Errorf("ALPS predict job is not implemented")
}

func (s *alpsExecutor) ExecuteEvaluate(stmt *ir.EvaluateStmt) (e error) {
	return fmt.Errorf("ALPS evaluate job is not implemented")
}

func (s *alpsExecutor) ExecuteExplain(stmt *ir.ExplainStmt) (e error) {
	return fmt.Errorf("ALPS explain job is not implemented")
}
