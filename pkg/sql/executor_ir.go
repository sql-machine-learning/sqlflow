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
	"strings"
	"time"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

// EndOfExecution will push to the pipe when one SQL statement execution is finished.
type EndOfExecution struct {
	StartTime int64
	EndTime   int64
	Statement string
}

// WorkflowJob indicates the Argo Workflow ID
type WorkflowJob struct {
	JobID string
}

// RunSQLProgram run a SQL program.
func RunSQLProgram(sqlProgram string, db *DB, modelDir string, session *pb.Session) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		err := runSQLProgram(wr, sqlProgram, db, modelDir, session)

		if err != nil {
			log.Errorf("runSQLProgram error: %v", err)
			if err != ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Errorf("runSQLProgram error(piping): %v", err)
				}
			}
		}
	}()
	return rd
}

// SubmitWorkflow submits an Argo workflow
func SubmitWorkflow(sqlProgram string, db *DB, modelDir string, session *pb.Session) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		err := submitWorkflow(wr, sqlProgram, db, modelDir, session)
		if err != nil {
			log.Errorf("submit Workflow error: %v", err)
		}
	}()
	return rd
}

func submitWorkflow(wr *PipeWriter, sqlProgram string, db *DB, modelDir string, session *pb.Session) error {
	sqls, err := parse(db.driverName, sqlProgram)
	if err != nil {
		return err
	}

	connStr := fmt.Sprintf("%s://%s", db.driverName, db.dataSourceName)

	_ = sqls
	_ = connStr
	/*
		_, err = programToIR(sqls, connStr, modelDir, true, false)
		if err != nil {
			return err
		}
	*/

	// TODO(yancey1989):
	// 1. call codegen_couler.go to genearte Couler program.
	// 2. compile Couler program into Argo YAML.
	// 3. submit Argo YAML and fetch the workflow ID.

	return wr.Write(WorkflowJob{
		JobID: "sqlflow-workflow",
	})
}

func runSQLProgram(wr *PipeWriter, sqlProgram string, db *DB, modelDir string, session *pb.Session) error {
	sqls, err := parse(db.driverName, sqlProgram)
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
		var r ir.SQLStatement
		connStr := fmt.Sprintf("%s://%s", db.driverName, db.dataSourceName)
		if sql.extended != nil {
			parsed := sql.extended
			if parsed.train {
				r, err = generateTrainIRWithInferredColumns(parsed, connStr)
			} else if parsed.analyze {
				r, err = generateAnalyzeIR(parsed, connStr, modelDir, submitter().GetTrainIRFromModel())
			} else {
				r, err = generatePredictIR(parsed, connStr, modelDir, submitter().GetTrainIRFromModel())
			}
		} else {
			standardSQL := ir.StandardSQL(sql.standard)
			r = &standardSQL
		}
		if err != nil {
			return err
		}
		r.SetOriginalSQL(sql.original)
		if e := runSingleSQLIR(wr, r, db, modelDir, session); e != nil {
			return e
		}
	}
	return nil
}

func runSingleSQLIR(wr *PipeWriter, sqlIR ir.SQLStatement, db *DB, modelDir string, session *pb.Session) (e error) {
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
	if e := submitter().Setup(wr, db, modelDir, session); e != nil {
		return e
	}
	defer submitter().Teardown()
	return sqlIR.Execute(submitter())
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(predParsed *extendedSelect, db *DB, session *pb.Session) error {
	tableName, columnName, e := parseTableColumn(predParsed.into)
	if e != nil {
		return fmt.Errorf("invalid predParsed.into, %v", e)
	}

	dropStmt := fmt.Sprintf("drop table if exists %s;", tableName)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	fts, e := verify(predParsed.standardSelect.String(), db)
	if e != nil {
		return e
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "create table %s (", tableName)
	for _, c := range predParsed.columns["feature_columns"] {
		name, err := getExpressionFieldName(c)
		if err != nil {
			return err
		}
		typ, ok := fts.get(name)
		if !ok {
			return fmt.Errorf("createPredictionTable: Cannot find type of field %s", name)
		}
		stype, e := universalizeColumnType(db.driverName, typ)
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "%s %s, ", name, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	typ, ok := fts.get(columnName)
	if !ok {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use INT type
		typ = "INT"
	}
	stype, e := universalizeColumnType(db.driverName, typ)
	if e != nil {
		return e
	}
	if db.driverName == "hive" {
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE;", columnName, stype)
	} else {
		fmt.Fprintf(&b, "%s %s);", columnName, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

// Create prediction table using the `PredictClause`.
// TODO(typhoonzero): remove legacy `createPredictionTable` once we change all submitters to use IR.
func createPredictionTableFromIR(predIR *ir.PredictClause, db *DB, session *pb.Session) error {
	dropStmt := fmt.Sprintf("drop table if exists %s;", predIR.ResultTable)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	// FIXME(typhoonzero): simply add LIMIT 1 at the end to get column types.
	tmpSQL := fmt.Sprintf("%s LIMIT 1;", strings.TrimRight(strings.TrimSpace(predIR.Select), ";"))
	flds, fts, e := getColumnTypes(tmpSQL, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	labelColumnTypeFound := false
	labelColumnName := ""
	labelColumnType := ""
	fmt.Fprintf(&b, "create table %s (", predIR.ResultTable)
	for idx, colType := range fts {
		stype, e := universalizeColumnType(db.driverName, colType)
		if e != nil {
			return e
		}
		fldName := flds[idx]
		if fldName == predIR.ResultColumn {
			labelColumnTypeFound = true
			labelColumnName = fldName
			labelColumnType = stype
			continue
		}
		fmt.Fprintf(&b, "%s %s, ", fldName, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	// typ, ok := fts.get(predIR.ResultColumn)
	if !labelColumnTypeFound {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use INT type
		labelColumnName = predIR.ResultColumn
		labelColumnType = "INT"
	}
	stype, e := universalizeColumnType(db.driverName, labelColumnType)
	if e != nil {
		return e
	}
	if db.driverName == "hive" {
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

func loadModelMeta(pr *extendedSelect, db *DB, cwd, modelDir, modelName string) (*extendedSelect, error) {
	var m *model
	var e error
	if modelDir != "" {
		m, e = loadTar(modelDir, cwd, modelName)
	} else {
		m, e = load(db, modelName, cwd)
	}
	if e != nil {
		return nil, fmt.Errorf("load %v", e)
	}
	// Parse the training SELECT statement used to train
	// the model for the prediction.
	tr, e := parseOneStatement(db.driverName, m.TrainSelect)
	if e != nil {
		return nil, fmt.Errorf("parse: TrainSelect %v raise %v", m.TrainSelect, e)
	}

	if e := verifyColumnNameAndType(tr.extended, pr, db); e != nil {
		return nil, fmt.Errorf("verifyColumnNameAndType: %v", e)
	}

	pr.trainClause = tr.extended.trainClause

	return pr, nil
}

func getDefaultSession() *pb.Session { return &pb.Session{} } // for testing
