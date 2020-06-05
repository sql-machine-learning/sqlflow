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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/log"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/step/feature"
	"sqlflow.org/sqlflow/pkg/verifier"
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
		err = runSQLProgram(wr, sqlProgram, db, modelDir, session)
		if err != nil {
			if e := wr.Write(fmt.Errorf("runSQLProgram error: %v", err)); e != nil {
				log.GetDefaultLogger().Errorf("runSQLProgram error(piping): %v", e)
			}
		}
	}()
	return rd
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
				// TODO(yancey1989): enable the atttribute checker when cover pai codegen.
				r, err = generateTrainStmt(sql.SQLFlowSelectStmt, false)
			} else if sql.ShowTrain {
				logger.Info("resolveSQL:showTrain")
				r, err = generateShowTrainStmt(sql.SQLFlowSelectStmt)
			} else if sql.Explain {
				logger.Info("resolveSQL:explain")
				// since getTrainStmtFromModel is false, use empty cwd is fine.
				r, err = generateExplainStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else if sql.Predict {
				logger.Info("resolveSQL:predict")
				r, err = generatePredictStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else if sql.Evaluate {
				logger.Info("resolveSQL:evaluate")
				r, err = generateEvaluateStmt(sql.SQLFlowSelectStmt, "", "", "", false)
			} else {
				return nil, fmt.Errorf("unkown exteneded SQL statement type")
			}
		} else {
			logger.Info("resolveSQL:standard")
			standardSQL := ir.NormalStmt(sql.Original)
			r = &standardSQL
		}
		if err != nil {
			return nil, err
		}
		r.SetOriginalSQL(sql.Original)
		logger.Infof("Original SQL is:%s", r.GetOriginalSQL())
		spIRs = append(spIRs, r)
	}
	return spIRs, nil
}

func runSQLProgram(wr *pipe.Writer, sqlProgram string, db *database.DB, modelDir string, session *pb.Session) error {
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
	cwd, err := ioutil.TempDir("", "sqlflow_models")
	if err != nil {
		return err
	}
	defer func(cwd string) {
		if err := os.RemoveAll(cwd); err != nil {
			e = fmt.Errorf("encounter %v when dealwith error: %s", e, err)
		}
	}(cwd)
	var r ir.SQLFlowStmt

	generateTrainStmtFromModel := GetSubmitter(session.Submitter).GetTrainStmtFromModel()

	if sql.IsExtendedSyntax() {
		if sql.Train {
			loadPreTrainModel := generateTrainStmtFromModel
			r, err = generateTrainStmtWithInferredColumns(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, loadPreTrainModel, true)
		} else if sql.ShowTrain {
			r, err = generateShowTrainStmt(sql.SQLFlowSelectStmt)
		} else if sql.Explain {
			r, err = generateExplainStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
		} else if sql.Predict {
			r, err = generatePredictStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
		} else if sql.Evaluate {
			r, err = generateEvaluateStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, generateTrainStmtFromModel)
		}
	} else {
		standardSQL := ir.NormalStmt(sql.Original)
		r = &standardSQL
	}
	if err != nil {
		return err
	}
	r.SetOriginalSQL(sql.Original)
	// TODO(typhoonzero): can run feature.LogDerivationResult(wr, trainStmt) here to send
	// feature derivation logs to client, yet we disable it for now so that it's less annoying.
	submitter := GetSubmitter(session.Submitter)
	submitter.Setup(wr, db, modelDir, cwd, session)
	return r.Execute(submitter)
}

// RewriteStatementsWithHints combines the hints into the standard SQL(s)
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
			hints += stmt.Original
		} else {
			sqls = append(sqls, stmt)
		}
	}
	return hints, sqls
}

func isHint(stmt *parser.SQLFlowStmt, dialect string) bool {
	if !stmt.IsExtendedSyntax() {
		if dialect == "alisa" {
			s := strings.ToLower(strings.TrimSpace(stmt.Original))
			if strings.HasPrefix(s, "set ") {
				return true
			}
		}
		// TODO(weiguoz) handle if submitter is "maxcompute" or "hive"
	}
	return false
}

// getColumnTypes is quiet like verify but accept a SQL string as input, and returns
// an ordered list of the field types.
func getColumnTypes(slct string, db *database.DB) ([]string, []string, error) {
	rows, err := feature.FetchSamples(db, slct)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, fmt.Errorf("query %s gives 0 row", slct)
	}

	if rows.Err() != nil {
		return nil, nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	ft := []string{}
	flds := []string{}
	for _, ct := range columnTypes {
		_, fld := verifier.Decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		flds = append(flds, fld)
		ft = append(ft, typeName)
	}

	return flds, ft, nil
}

// Create prediction table using the `PredictStmt`.
func createPredictionTableFromIR(predStmt *ir.PredictStmt, db *database.DB, session *pb.Session) error {
	dropStmt := fmt.Sprintf("drop table if exists %s;", predStmt.ResultTable)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	flds, fts, e := getColumnTypes(predStmt.Select, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	// NOTE(typhoonzero): predStmt.TrainStmt may be nil, because the model may not loaded when
	// creating prediction table.
	trainLabelColumn := ""
	if predStmt.TrainStmt != nil {
		trainLabelColumn = predStmt.TrainStmt.Label.GetFieldDesc()[0].Name
	}
	resultColumnName := predStmt.ResultColumn
	resultColumnType := ""
	fmt.Fprintf(&b, "create table %s (", predStmt.ResultTable)
	for idx, colType := range fts {
		stype, e := fieldType(db.DriverName, colType)
		if e != nil {
			return e
		}
		fldName := flds[idx]
		// When predicting use validation table, we should find the label column type
		// using the label column name from train table.
		if fldName == trainLabelColumn {
			resultColumnType = stype
			if resultColumnName == trainLabelColumn {
				continue
			}
		}
		// result column have the same name, do not add as feature column
		if fldName == resultColumnName {
			resultColumnType = stype
			continue
		}
		fmt.Fprintf(&b, "%s %s, ", fldName, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	//
	// label column not found in predict table, create a column specified by PREDICT clause:
	if resultColumnType == "" {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use INT type
		resultColumnType = "INT"
	}
	stype, e := fieldType(db.DriverName, resultColumnType)
	if e != nil {
		return e
	}
	if db.DriverName == "hive" {
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE;", resultColumnName, stype)
	} else {
		fmt.Fprintf(&b, "%s %s);", resultColumnName, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}
