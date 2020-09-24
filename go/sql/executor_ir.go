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

package sql

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"sqlflow.org/sqlflow/go/codegen/optimize"
	"sqlflow.org/sqlflow/go/codegen/pai"
	"sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/codegen/xgboost"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/executor"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/pipe"
	pb "sqlflow.org/sqlflow/go/proto"
)

// EndOfExecution will push to the pipe when one SQL statement execution is finished.
type EndOfExecution struct {
	StartTime int64
	EndTime   int64
	Statement string
}

// RunSQLProgram run a SQL program.
//
// TODO(wangkuiyi): Make RunSQLProgram return an error in addition to
// *pipe.Reader, and remove the calls to log.Printf.
func RunSQLProgram(sqlProgram string, modelDir string, session *pb.Session) *pipe.Reader {
	rd, wr := pipe.Pipe()
	go func() {
		var db *database.DB
		var err error
		defer wr.Close()
		if db, err = database.OpenAndConnectDB(session.DbConnStr); err != nil {
			wr.Write(fmt.Errorf("create DB failed: %v", err))
			return
		}
		defer db.Close()
		err = runSQLProgram(wr, sqlProgram, db, modelDir, session)
		if err != nil {
			if e := wr.Write(fmt.Errorf("runSQLProgram error: %v", err)); e != nil {
				log.GetDefaultLogger().Errorf("runSQLProgram error(piping): %v", e)
			}
		}
	}()
	return rd
}

func initializeAndCheckAttributes(stmt ir.SQLFlowStmt) error {
	switch s := stmt.(type) {
	case *ir.TrainStmt:
		if s.GetModelKind() == ir.XGBoost {
			return xgboost.InitializeAttributes(s)
		} else if s.GetModelKind() == ir.KMeans {
			return pai.InitializeKMeansAttributes(s)
		}
		return tensorflow.InitializeAttributes(s)
	case *ir.OptimizeStmt:
		return optimize.InitializeAttributes(s)
	}
	return nil
}

// ResolveSQLProgram accepts parse result from parser and returns a list of SQLFlowStmt
func ResolveSQLProgram(sqlStmts []*parser.SQLFlowStmt, logger *log.Logger) ([]ir.SQLFlowStmt, error) {
	spIRs := []ir.SQLFlowStmt{}
	var err error
	for _, sql := range sqlStmts {
		var r ir.SQLFlowStmt
		if sql.IsExtendedSyntax() {
			if sql.Train {
				logger.Info("resolveSQL:train")
				r, err = ir.GenerateTrainStmt(sql.SQLFlowSelectStmt)
			} else if sql.ShowTrain {
				logger.Info("resolveSQL:showTrain")
				r, err = ir.GenerateShowTrainStmt(sql.SQLFlowSelectStmt)
			} else if sql.Explain {
				logger.Info("resolveSQL:explain")
				// since getTrainStmtFromModel is false, use empty cwd is fine.
				r, err = ir.GenerateExplainStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else if sql.Predict {
				logger.Info("resolveSQL:predict")
				r, err = ir.GeneratePredictStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else if sql.Evaluate {
				logger.Info("resolveSQL:evaluate")
				r, err = ir.GenerateEvaluateStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else if sql.Optimize {
				logger.Info("resolveSQL:optimize")
				r, err = ir.GenerateOptimizeStmt(sql.SQLFlowSelectStmt)
			} else if sql.Run {
				logger.Info("resolveSQL:run")
				r, err = ir.GenerateRunStmt(sql.SQLFlowSelectStmt)
			} else {
				return nil, fmt.Errorf("unknown extended SQL statement type")
			}
		} else {
			logger.Info("resolveSQL:standard")
			standardSQL := ir.NormalStmt(sql.Original)
			r = &standardSQL
		}
		if err != nil {
			return nil, err
		}
		// TODO(yancey1989): enable the attribute checker when cover pai codegen.
		// if err = initializeAndCheckAttributes(r); err != nil {
		// 	return nil, err
		// }
		r.SetOriginalSQL(sql.Original)
		logger.Infof("Original SQL is:%s", r.GetOriginalSQL())
		spIRs = append(spIRs, r)
	}
	return spIRs, nil
}

func runSQLProgram(wr *pipe.Writer, sqlProgram string, db *database.DB, modelDir string, session *pb.Session) error {
	sqlProgram, err := parser.RemoveCommentInSQLStatement(sqlProgram)
	if err != nil {
		return err
	}

	stmts, err := parser.Parse(db.DriverName, sqlProgram)
	if err != nil {
		return err
	}
	// NOTE(tony): We generate IR and execute its translated program one-by-one since IR generation may depend on the execution
	// of the previous statement. For example, consider a SQL program
	//
	//   create table some_table as (select ...);
	//   select * from some_table to train ...
	//
	// The IR generation on the second statement would fail since it requires inspection the schema of some_table,
	// which depends on the execution of create table some_table as (select ...);.
	sqls := RewriteStatementsWithHints(stmts, db.DriverName)
	for _, sql := range sqls {
		if err := runSingleSQLFlowStatement(wr, sql, db, modelDir, session); err != nil {
			return err
		}
	}
	return nil
}

func runSingleSQLFlowStatement(wr *pipe.Writer, sql *parser.SQLFlowStmt, db *database.DB, modelDir string, session *pb.Session) (e error) {
	defer func(startTime int64) {
		// NOTE(tony): EndOfExecution indicates a successful run,
		// so we only writes it when e != nil
		if e != nil {
			wr.Write(EndOfExecution{
				StartTime: startTime,
				EndTime:   time.Now().UnixNano(),
				Statement: sql.Original,
			})
		}
	}(time.Now().UnixNano())

	// use system default tmp dir
	cwd, err := ioutil.TempDir("/tmp", "sqlflow_models")
	if err != nil {
		return err
	}
	defer func(cwd string) {
		if err := os.RemoveAll(cwd); err != nil {
			e = fmt.Errorf("encounter an error when removing temp files: %v", err)
		}
	}(cwd)

	var r ir.SQLFlowStmt
	if sql.IsExtendedSyntax() {
		generateTrainStmtFromModel := executor.New(session.Submitter).GetTrainStmtFromModel()
		if sql.Train {
			// generateTrainStmtFromModel refers to if a pre-trained model
			r, err = ir.GenerateTrainStmtWithInferredColumns(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel, true)
		} else if sql.ShowTrain {
			r, err = ir.GenerateShowTrainStmt(sql.SQLFlowSelectStmt)
		} else if sql.Explain {
			r, err = ir.GenerateExplainStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
		} else if sql.Predict {
			r, err = ir.GeneratePredictStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
		} else if sql.Evaluate {
			r, err = ir.GenerateEvaluateStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
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
		return err
	}
	if err = initializeAndCheckAttributes(r); err != nil {
		return err
	}
	r.SetOriginalSQL(sql.Original)
	// TODO(typhoonzero): can run feature.LogDerivationResult(wr, trainStmt) here to send
	// feature derivation logs to client, yet we disable it for now so that it's less annoying.

	exec := executor.New(session.Submitter)
	exec.Setup(wr, db, modelDir, cwd, session)
	return executor.Run(exec, r)
}

// RewriteStatementsWithHints combines the hints into the standard SQL(s)
//
// FIXME(weiguoz): I'm not happy with such an implementation.
// I mean it is not clean that sqlflow handles such database relative details.
func RewriteStatementsWithHints(stmts []*parser.SQLFlowStmt, dialect string) []*parser.SQLFlowStmt {
	hints, sqls := splitHints(stmts, dialect)
	if len(hints) > 0 {
		for _, sql := range sqls {
			if !sql.IsExtendedSyntax() {
				sql.Original = hints + sql.Original
			}
		}
	}
	return sqls
}

func splitHints(stmts []*parser.SQLFlowStmt, dialect string) (string, []*parser.SQLFlowStmt) {
	hints, sqls := "", []*parser.SQLFlowStmt{}
	for _, stmt := range stmts {
		if isHint(stmt, dialect) {
			hints += stmt.Original + "\n" // alisa's requirements
		} else {
			sqls = append(sqls, stmt)
		}
	}
	return hints, sqls
}

func isHint(stmt *parser.SQLFlowStmt, dialect string) bool {
	if !stmt.IsExtendedSyntax() {
		if dialect == "alisa" {
			return isAlisaHint(stmt.Original)
		}
		// TODO(weiguoz) handle if submitter is "maxcompute" or "hive"
	}
	return false
}

func isAlisaHint(sql string) bool {
	for {
		sql = strings.TrimSpace(sql)
		// TODO(weiguoz): Let's remove the following code if we clean the comments before
		if strings.HasPrefix(sql, "--") {
			eol := strings.IndexAny(sql, "\n\r")
			if eol != -1 {
				sql = sql[eol+1:]
			} else {
				break
			}
		} else {
			break
		}
	}
	return strings.HasPrefix(strings.ToLower(sql), "set ")
}
