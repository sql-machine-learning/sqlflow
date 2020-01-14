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
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"sqlflow.org/sqlflow/pkg/argo"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/couler"
	"sqlflow.org/sqlflow/pkg/sql/ir"
	"sqlflow.org/sqlflow/pkg/verifier"
)

// EndOfExecution will push to the pipe when one SQL statement execution is finished.
type EndOfExecution struct {
	StartTime int64
	EndTime   int64
	Statement string
}

// WorkflowJob indicates the Argo Workflow ID
// FIXME(tony): reuse workflow job definition in proto package
type WorkflowJob struct {
	JobID string
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
			log.Printf("create DB failed: %v", err)
			return
		}
		err = runSQLProgram(wr, sqlProgram, db, modelDir, session)
		if err != nil {
			log.Printf("runSQLProgram error: %v", err)
			if err != pipe.ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Printf("runSQLProgram error(piping): %v", err)
				}
			}
		}
	}()
	return rd
}

// SubmitWorkflow submits an Argo workflow
//
// TODO(wangkuiyi): Make RunSQLProgram return an error in addition to
// *pipe.Reader, and remove the calls to log.Printf.
func SubmitWorkflow(sqlProgram string, modelDir string, session *pb.Session) *pipe.Reader {
	rd, wr := pipe.Pipe()
	go func() {
		defer wr.Close()
		err := submitWorkflow(wr, sqlProgram, modelDir, session)
		if err != nil {
			if err != pipe.ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Printf("submit workflow error(piping): %v", err)
				}
			}
		}
	}()
	return rd
}

func submitWorkflow(wr *pipe.Writer, sqlProgram string, modelDir string, session *pb.Session) error {
	driverName, _, err := database.ParseURL(session.DbConnStr)
	if err != nil {
		return err
	}
	sqls, err := parser.Parse(driverName, sqlProgram)
	if err != nil {
		return err
	}
	// TODO(yancey1989): separate the IR generation to multiple steps:
	// For example, a TRAIN statement:
	// 		SELECT ... TO TRAIN ...
	// the multiple ir generator steps pipeline can be:
	// sql -> parsed result -> infer columns -> load train ir from saved model ..
	spIRs := []ir.SQLStatement{}
	for _, sql := range sqls {
		var r ir.SQLStatement
		if parser.IsExtendedSyntax(sql) {
			if sql.Train {
				r, err = generateTrainStmt(sql.SQLFlowSelectStmt)
			} else if sql.Explain {
				// since getTrainStmtFromModel is false, use empty cwd is fine.
				r, err = generateExplainStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, "", false)
			} else {
				r, err = generatePredictStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, "", false)
			}
		} else {
			standardSQL := ir.StandardSQL(sql.Standard)
			r = &standardSQL
		}
		if err != nil {
			return err
		}
		r.SetOriginalSQL(sql.Original)
		spIRs = append(spIRs, r)
	}

	// 1. call codegen_couler.go to generate Argo workflow YAML
	argoFileName, err := couler.RunAndWriteArgoFile(spIRs, session)
	if err != nil {
		return err
	}
	defer os.RemoveAll(argoFileName)

	// 2. submit the argo workflow
	workflowID, err := argo.Submit(argoFileName)
	if err != nil {
		return err
	}

	return wr.Write(WorkflowJob{
		JobID: workflowID,
	})
}

func runSQLProgram(wr *pipe.Writer, sqlProgram string, db *database.DB, modelDir string, session *pb.Session) error {
	sqls, err := parser.Parse(db.DriverName, sqlProgram)
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
	for _, sql := range sqls {
		cwd, err := ioutil.TempDir("/tmp", "sqlflow_models")
		if err != nil {
			return err
		}
		// NOTE(typhoonzero): must call "cleanCwd" when end processing current SQL or before
		// returning error, we can not use "defer" because if we have many SQL statements in
		// the SQL program, we may overflow the defer stack.
		// For more information: https://blog.learngoprogramming.com/gotchas-of-defer-in-go-1-8d070894cb01
		cleanCwd := func(cwd string) error {
			return os.RemoveAll(cwd)
		}
		var r ir.SQLStatement
		if parser.IsExtendedSyntax(sql) {
			if sql.Train {
				r, err = generateTrainStmtWithInferredColumns(sql.SQLFlowSelectStmt, session.DbConnStr, true)
			} else if sql.Explain {
				r, err = generateExplainStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, GetSubmitter(session.Submitter).GetTrainStmtFromModel())
			} else {
				r, err = generatePredictStmt(sql.SQLFlowSelectStmt, session.DbConnStr, modelDir, cwd, GetSubmitter(session.Submitter).GetTrainStmtFromModel())
			}
		} else {
			standardSQL := ir.StandardSQL(sql.Standard)
			r = &standardSQL
		}

		if err != nil {
			if e := cleanCwd(cwd); e != nil {
				return fmt.Errorf("encounter %v when dealwith error: %s", e, err)
			}
			return err
		}
		r.SetOriginalSQL(sql.Original)
		if err := runSingleSQLIR(wr, r, db, modelDir, cwd, session); err != nil {
			if e := cleanCwd(cwd); e != nil {
				return fmt.Errorf("encounter %v when dealwith error: %s", e, err)
			}
			return err
		}
		if e := cleanCwd(cwd); e != nil {
			return fmt.Errorf("encounter %v when dealwith error: %s", e, err)
		}
	}
	return nil
}

func runSingleSQLIR(wr *pipe.Writer, sqlIR ir.SQLStatement, db *database.DB, modelDir string, cwd string, session *pb.Session) (e error) {
	startTime := time.Now().UnixNano()
	var originalSQL string
	defer func() {
		if e != nil {
			wr.Write(EndOfExecution{
				StartTime: startTime,
				EndTime:   time.Now().UnixNano(),
				Statement: originalSQL,
			})
		}
	}()
	// TODO(typhoonzero): can run feature.LogDerivationResult(wr, trainStmt) here to send
	// feature derivation logs to client, yet we disable if for now so that it's less annoying.
	GetSubmitter(session.Submitter).Setup(wr, db, modelDir, cwd, session)
	return sqlIR.Execute(GetSubmitter(session.Submitter))
}

// getColumnTypes is quiet like verify but accept a SQL string as input, and returns
// an ordered list of the field types.
func getColumnTypes(slct string, db *database.DB) ([]string, []string, error) {
	rows, err := db.Query(slct)
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
	// FIXME(typhoonzero): simply add LIMIT 1 at the end to get column types.
	tmpSQL := fmt.Sprintf("%s LIMIT 1;", strings.TrimRight(strings.TrimSpace(predStmt.Select), ";"))
	flds, fts, e := getColumnTypes(tmpSQL, db)
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
	labelColumnName := predStmt.ResultColumn
	labelColumnType := ""
	fmt.Fprintf(&b, "create table %s (", predStmt.ResultTable)
	for idx, colType := range fts {
		stype, e := fieldType(db.DriverName, colType)
		if e != nil {
			return e
		}
		fldName := flds[idx]
		// When predicting use validation table, we should find the label column type
		// using the label column name from train table.
		if fldName == labelColumnName || fldName == trainLabelColumn {
			labelColumnType = stype
			continue
		}
		fmt.Fprintf(&b, "%s %s, ", fldName, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	//
	// label column not found in predict table, create a column specified by PREDICT clause:
	if labelColumnType == "" {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use INT type
		labelColumnType = "INT"
	}
	stype, e := fieldType(db.DriverName, labelColumnType)
	if e != nil {
		return e
	}
	if db.DriverName == "hive" {
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE;", labelColumnName, stype)
	} else {
		fmt.Fprintf(&b, "%s %s);", labelColumnName, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}
