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
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	pb "sqlflow.org/sqlflow/pkg/server/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
)

// EndOfExecution will push to the pipe when one SQL statement execution is finished.
type EndOfExecution struct {
	StartTime int64
	EndTime   int64
	Statement string
}

// RunSQLProgram run a SQL program.
func RunSQLProgram(sqlProgram string, db *DB, modelDir string, session *pb.Session) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		err := runSQLProgram(wr, sqlProgram, db, modelDir, session)

		if err != nil {
			log.Errorf("runSQLProgram error:%v", err)
			if err != ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Errorf("runSQLProgram error(piping):%v", err)
				}
			}
		}
	}()
	return rd
}

func runSQLProgram(wr *PipeWriter, sqlProgram string, db *DB, modelDir string, session *pb.Session) error {
	sqls, err := SplitMultipleSQL(sqlProgram)
	if err != nil {
		return err
	}

	connStr := fmt.Sprintf("%s://%s", db.driverName, db.dataSourceName)
	programIR, err := programToIR(sqls, connStr, modelDir)
	if err != nil {
		return err
	}

	for _, ir := range programIR {
		if e := runSingleSQLIR(wr, ir, db, modelDir, session); e != nil {
			return e
		}
	}

	return nil
}

func runSingleSQLIR(wr *PipeWriter, ir codegen.SingleSQLIR, db *DB, modelDir string, session *pb.Session) (e error) {
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

	switch ir.(type) {
	case *codegen.StandardSQLIR:
		originalSQL = string(*ir.(*codegen.StandardSQLIR))
		if e = runStandardSQL(wr, originalSQL, db); e != nil {
			return e
		}
	case *codegen.TrainIR:
		originalSQL = ir.(*codegen.TrainIR).OriginalSQL
		if e = runTrainIR(ir.(*codegen.TrainIR), wr, db, modelDir, session); e != nil {
			return e
		}
	case *codegen.PredictIR:
		originalSQL = ir.(*codegen.PredictIR).OriginalSQL
		if e = runPredictIR(ir.(*codegen.PredictIR), wr, db, modelDir, session); e != nil {
			return e
		}
	case *codegen.AnalyzeIR:
		originalSQL = ir.(*codegen.AnalyzeIR).OriginalSQL
		if e = runAnalyzeIR(ir.(*codegen.AnalyzeIR), wr, db, modelDir, session); e != nil {
			return e
		}
	default:
		return fmt.Errorf("got error ir type: %T", ir)
	}

	return nil
}

func runTrainIR(trainIR *codegen.TrainIR, wr *PipeWriter, db *DB, modelDir string, session *pb.Session) error {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	pr, e := newParser().Parse(trainIR.OriginalSQL)
	if e != nil {
		return e
	}
	// cwd is used to store train scripts and save output models.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cwd)

	if os.Getenv("SQLFLOW_submitter") == "elasticdl" {
		return elasticDLTrain(wr, pr, db, cwd, session)
	}
	// FIXME(weiguo): temporary branch to alps
	if os.Getenv("SQLFLOW_submitter") == "alps" {
		return alpsTrain(wr, pr, db, cwd, session)
	}
	// ---------------------- run the IR ---------------------------
	var program bytes.Buffer
	if isXGBoostModel(trainIR.Estimator) {
		err := InferFeatureColumns(trainIR)
		if err != nil {
			return err
		}
		code, err := xgboost.Train(trainIR)
		if err != nil {
			return err
		}
		program.WriteString(code)
	} else {
		err := InferFeatureColumns(trainIR)
		if err != nil {
			return err
		}
		if trainIR.ValidationSelect == "" {
			trainIR.ValidationSelect = trainIR.Select
		}
		code, err := tensorflow.Train(trainIR)
		if err != nil {
			return err
		}
		program.WriteString(code)
	}
	cw := &logChanWriter{wr: wr}
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("\n==========Program======\n%s\n=======Program Output===========\n", program.String()))

	w := io.MultiWriter(cw, &buf)
	defer cw.Close()
	cmd := sqlflowCmd(cwd, db.driverName)
	cmd.Stdin = &program
	cmd.Stdout = w
	cmd.Stderr = w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("predict failed: %v\n %s", e, buf.String())
	}
	m := model{workDir: cwd, TrainSelect: trainIR.OriginalSQL}
	if modelDir != "" {
		return m.saveTar(modelDir, pr.save)
	}
	return m.save(db, pr.save)
}

func runPredictIR(predIR *codegen.PredictIR, wr *PipeWriter, db *DB, modelDir string, session *pb.Session) error {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	pr, e := newParser().Parse(predIR.OriginalSQL)
	if e != nil {
		return e
	}
	// cwd is used to load the saved model for prediction.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cwd)

	if os.Getenv("SQLFLOW_submitter") == "alps" {
		return alpsPred(wr, pr, db, cwd, session)
	} else if os.Getenv("SQLFLOW_submitter") == "elasticdl" {
		return elasticDLPredict(wr, pr, db, cwd, session)
	}
	// ------------------- run pred IR -----------------------
	// TODO(typhoonzero): loadModelMeta should use IR
	pr, _, e = loadModelMeta(pr, db, cwd, modelDir, pr.model)
	if e != nil {
		return fmt.Errorf("loadModelMeta %v", e)
	}

	var program bytes.Buffer
	if isXGBoostModel(predIR.TrainIR.Estimator) {
		err = InferFeatureColumns(predIR.TrainIR)
		if err != nil {
			return err
		}
		code, err := xgboost.Pred(predIR, session)
		if err != nil {
			return err
		}
		err = createPredictionTableFromIR(predIR, db, session)
		if err != nil {
			return err
		}
		program.WriteString(code)
	} else {
		err = InferFeatureColumns(predIR.TrainIR)
		if err != nil {
			return err
		}
		code, err := tensorflow.Pred(predIR, session)
		if err != nil {
			return err
		}
		err = createPredictionTableFromIR(predIR, db, session)
		if err != nil {
			return err
		}
		program.WriteString(code)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("\n==========Program======\n%s\n=======Program Output===========\n", program.String()))

	cw := &logChanWriter{wr: wr}
	w := io.MultiWriter(cw, &buf)
	defer cw.Close()
	cmd := sqlflowCmd(cwd, db.driverName)
	cmd.Env = append(os.Environ())
	cmd.Stdin = &program
	cmd.Stdout = w
	cmd.Stderr = w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("predict failed: %v\n %s", e, buf.String())
	}
	return nil
}

func runAnalyzeIR(analyzeIR *codegen.AnalyzeIR, wr *PipeWriter, db *DB, modelDir string, session *pb.Session) error {
	pr, e := newParser().Parse(analyzeIR.OriginalSQL)
	if e != nil {
		return e
	}
	// cwd is used to load the saved model for prediction.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cwd)
	// load the model for analyze
	pr, _, e = loadModelMeta(pr, db, cwd, modelDir, pr.trainedModel)
	if e != nil {
		return fmt.Errorf("loadModelMeta %v", e)
	}

	cmd := exec.Command("python", "-u")
	cmd.Dir = cwd

	if !strings.HasPrefix(strings.ToUpper(analyzeIR.TrainIR.Estimator), `XGBOOST.`) {
		return fmt.Errorf("unsupported model %s", analyzeIR.TrainIR.Estimator)
	}
	code, err := xgboost.Analyze(analyzeIR)
	if err != nil {
		return err
	}
	var program bytes.Buffer
	program.WriteString(code)
	cmd.Stdin = &program
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	imgFile, err := os.Open(path.Join(cwd, "summary.png"))
	if err != nil {
		return err
	}
	defer imgFile.Close()

	imgBytes, err := ioutil.ReadAll(imgFile)
	if err != nil {
		return err
	}
	imgBase64Str := base64.StdEncoding.EncodeToString(imgBytes)
	img2html := fmt.Sprintf("<div align='center'><img src='data:image/png;base64,%s' /></div>", imgBase64Str)
	wr.Write(img2html)
	return nil
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

	fts, e := verify(predParsed, db)
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

// Create prediction table using the `PredictIR`.
// TODO(typhoonzero): remove legacy `createPredictionTable` once we change all submitters to use IR.
func createPredictionTableFromIR(predIR *codegen.PredictIR, db *DB, session *pb.Session) error {
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

func loadModelMeta(pr *extendedSelect, db *DB, cwd, modelDir, modelName string) (*extendedSelect, fieldTypes, error) {
	var m *model
	var e error
	if modelDir != "" {
		m, e = loadTar(modelDir, cwd, modelName)
	} else {
		m, e = load(db, modelName, cwd)
	}
	if e != nil {
		return nil, nil, fmt.Errorf("load %v", e)
	}

	// Parse the training SELECT statement used to train
	// the model for the prediction.
	tr, e := newParser().Parse(m.TrainSelect)
	if e != nil {
		return nil, nil, fmt.Errorf("parse: TrainSelect %v raise %v", m.TrainSelect, e)
	}

	if e := verifyColumnNameAndType(tr, pr, db); e != nil {
		return nil, nil, fmt.Errorf("verifyColumnNameAndType: %v", e)
	}

	pr.trainClause = tr.trainClause
	fts, e := verify(pr, db)
	if e != nil {
		return nil, nil, fmt.Errorf("verify: %v", e)
	}

	return pr, fts, nil
}

type logChanWriter struct {
	wr *PipeWriter

	m    sync.Mutex
	buf  bytes.Buffer
	prev string
}

func (cw *logChanWriter) Write(p []byte) (n int, err error) {
	// Both cmd.Stdout and cmd.Stderr are writing to cw
	cw.m.Lock()
	defer cw.m.Unlock()

	n, err = cw.buf.Write(p)
	if err != nil {
		return n, err
	}

	for {
		line, err := cw.buf.ReadString('\n')
		cw.prev = cw.prev + line
		// ReadString returns err != nil if and only if the returned Data
		// does not end in delim.
		if err != nil {
			break
		}

		if err := cw.wr.Write(cw.prev); err != nil {
			return len(cw.prev), err
		}
		cw.prev = ""
	}
	return n, nil
}

func (cw *logChanWriter) Close() {
	if len(cw.prev) > 0 {
		cw.wr.Write(cw.prev)
		cw.prev = ""
	}
}

// ----------------------- useful for testing --------------------------

func getDefaultSession() *pb.Session {
	return &pb.Session{}
}

func errorPipe(err error) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		wr.Write(err)
	}()
	return rd
}
