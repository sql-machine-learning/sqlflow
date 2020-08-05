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
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/parser"
	pb "sqlflow.org/sqlflow/go/proto"
)

// GenerateCodeCouler generate a Couler program to submit a workflow to run the sql program.
// 1. generate IR of each statement.
// 2. generate runtime code of each statement
// 3. generate couler program to form a workflow
func GenerateCodeCouler(sqlProgram string, session *pb.Session) (string, error) {
	stmts, err := parseToIR(sqlProgram, session)
	if err != nil {
		return "", err
	}
	for _, stmt := range stmts {
		stepCode, err := generateStepCode(stmt, session)
		if err != nil {
			return "", err
		}
		fmt.Println(stepCode)
	}
	return "", nil
}

func parseToIR(sqlProgram string, session *pb.Session) ([]ir.SQLFlowStmt, error) {
	var dbDriver string
	var r ir.SQLFlowStmt
	var result []ir.SQLFlowStmt

	sqlProgram, err := parser.RemoveCommentInSQLStatement(sqlProgram)
	if err != nil {
		return nil, err
	}

	dbDriverParts := strings.Split(session.DbConnStr, "://")
	if len(dbDriverParts) != 2 {
		return nil, fmt.Errorf("invalid database connection string %s", session.DbConnStr)
	}
	dbDriver = dbDriverParts[0]

	stmts, err := parser.Parse(dbDriver, sqlProgram)
	if err != nil {
		return nil, err
	}
	sqls := rewriteStatementsWithHints(stmts, dbDriver)
	for _, sql := range sqls {
		if sql.IsExtendedSyntax() {
			if sql.Train {
				// TODO(typhoonzero): use feature derivation at runtime, call GenerateTrainStmt only.
				r, err = ir.GenerateTrainStmtWithInferredColumns(sql.SQLFlowSelectStmt, session.DbConnStr, "", "", false, false)
			} else if sql.ShowTrain {
				r, err = ir.GenerateShowTrainStmt(sql.SQLFlowSelectStmt)
			} else if sql.Explain {
				r, err = ir.GenerateExplainStmt(sql.SQLFlowSelectStmt, session.DbConnStr, "", "", false)
			} else if sql.Predict {
				r, err = ir.GeneratePredictStmt(sql.SQLFlowSelectStmt, session.DbConnStr, "", "", false)
			} else if sql.Evaluate {
				r, err = ir.GenerateEvaluateStmt(sql.SQLFlowSelectStmt, session.DbConnStr, "", "", false)
			} else if sql.Optimize {
				r, err = ir.GenerateOptimizeStmt(sql.SQLFlowSelectStmt)
			} else if sql.Run {
				r, err = ir.GenerateRunStmt(sql.SQLFlowSelectStmt)
			}
		} else {
			standardSQL := ir.NormalStmt(sql.Original)
			r = &standardSQL
		}
		if err != nil {
			return nil, err
		}
		if err = initializeAndCheckAttributes(r); err != nil {
			return nil, err
		}
		r.SetOriginalSQL(sql.Original)
		result = append(result, r)
	}
	return result, nil
}

func generateStepCode(stmt ir.SQLFlowStmt, session *pb.Session) (string, error) {
	switch stmt.(type) {
	case *ir.TrainStmt:
		trainStmt := stmt.(*ir.TrainStmt)
		if strings.HasPrefix(strings.ToUpper(trainStmt.Estimator), "XGBOOST.") {
			return XGBoostGenerateTrain(trainStmt, session)
		}
		return "", fmt.Errorf("not implemented estimator type %s", trainStmt.Estimator)
	default:
		return "", fmt.Errorf("not implemented stmt execution type %v", stmt)
	}
}

func initializeAndCheckAttributes(stmt ir.SQLFlowStmt) error {
	switch s := stmt.(type) {
	case *ir.TrainStmt:
		if s.GetModelKind() == ir.XGBoost {
			return InitializeAttributes(s)
		}
		// TODO(typhoonzero): add below lines
		// 	else if s.GetModelKind() == ir.KMeans {
		// 		return pai.InitializeKMeansAttributes(s)
		// 	}
		// 	return tensorflow.InitializeAttributes(s)
		// case *ir.OptimizeStmt:
		// 	return optimize.InitializeAttributes(s)
	}
	return nil
}

// InitializeAttributes initializes the attributes of XGBoost and does type checking for them
func InitializeAttributes(trainStmt *ir.TrainStmt) error {
	attributeDictionary.ExportDefaults(trainStmt.Attributes)
	return fullAttrValidator.Validate(trainStmt.Attributes)
}
