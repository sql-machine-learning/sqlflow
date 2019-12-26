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
	"fmt"
	"os"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type requestContext struct {
	SQLProgram string
	Session    *pb.Session

	ModelSaveDir string // directory for save/load model, empty if we are going to save model to db

	Wr            *pipe.Writer
	ProgramIR     ir.SQLProgram // parsed
	Conn          *database.DB  // connection to database for current request
	Cwd           string        // current working directory, will generate python code, save model, load model to this path
	IsModelLoaded bool          // true if model is loaded into Cwd

	Submitter Submitter // a proper submitter for current request
}

func newRequest(wr *pipe.Writer, sqlProgram string, session *pb.Session, conn *database.DB, modelSaveDir string) *requestContext {
	submitter := getSubmitter()
	req := &requestContext{
		SQLProgram:   sqlProgram,
		Conn:         conn,
		Session:      session,
		ModelSaveDir: modelSaveDir,
		Submitter:    submitter,
	}
	return req
}

func (req *requestContext) close() error {
	if err := os.RemoveAll(req.Cwd); err != nil {
		return err
	}
	return nil
}

func (req *requestContext) executeSQL(sqlIR ir.SQLStatement) error {
	var err error
	switch sqlIR.(type) {
	case *ir.TrainStmt:
		err = req.Submitter.ExecuteTrain(sqlIR.(*ir.TrainStmt), req)
	case *ir.PredictStmt:
		err = req.Submitter.ExecutePredict(sqlIR.(*ir.PredictStmt), req)
	case *ir.AnalyzeStmt:
		err = req.Submitter.ExecuteAnalyze(sqlIR.(*ir.AnalyzeStmt), req)
	case *ir.StandardSQL:
		err = req.Submitter.ExecuteQuery(sqlIR.(*ir.StandardSQL), req)
	default:
		return fmt.Errorf("not supported ir.SQLStatement: %v", sqlIR)
	}
	return err
}
