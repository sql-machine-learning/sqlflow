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
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
)

// Run executes a SQL query and returns a stream of rows or messages
func Run(slct string, db *DB, modelDir string, session *pb.Session) *PipeReader {
	splittedSQL, err := splitExtendedSQL(slct)
	if err != nil {
		rd, wr := Pipe()
		// return the lexer error message to client side
		go func() {
			defer wr.Close()
			wr.Write(err)
		}()
		return rd
	}
	if len(splittedSQL) == 2 {
		return runExtendedSQL(slct, db, modelDir, session)
	}
	return runStandardSQL(slct, db)
}

// splitExtendedSQL splits an extended select statement into
// its select clause and the rest. For example,
//
// input:
//   "select ... train ... with ..."
// output:
//   ["select ...", "train ... with ..."].
//
// input:
//   "select ... predict ... using ..."
// output:
//   ["select ...", "predict ... using ..."].
//
// input:
//   "select ..."
// output:
//   ["select ..."]
func splitExtendedSQL(slct string) ([]string, error) {
	l := newLexer(slct)
	var n sqlSymType
	var typ []int
	var pos []int
	for {
		t := l.Lex(&n)
		if t < 0 {
			return []string{}, fmt.Errorf("Lex: Unknown problem %s", slct[0-t:])
		}
		if t == 0 {
			break
		}
		typ = append(typ, t)
		pos = append(pos, l.pos)
	}
	for i := 1; i < len(typ)-2; i++ {
		if (typ[i] == TRAIN && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == PREDICT && typ[i+1] == IDENT && typ[i+2] == USING) ||
			(typ[i] == PREDICT && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == ANALYZE && typ[i+1] == IDENT && typ[i+2] == WITH) ||
			(typ[i] == ANALYZE && typ[i+1] == IDENT && typ[i+2] == USING) {
			return []string{slct[:pos[i-1]], slct[pos[i-1]:]}, nil
		}
	}

	return []string{slct}, nil
}

// TODO(weiguo): isQuery is a hacky way to decide which API to call:
// https://golang.org/pkg/database/sql/#DB.Exec .
// We will need to extend our parser to be a full SQL parser in the future.
func isQuery(slct string) bool {
	s := strings.ToUpper(strings.TrimSpace(slct))
	has := strings.Contains
	if strings.HasPrefix(s, "SELECT") && !has(s, "INTO") {
		return true
	}
	if strings.HasPrefix(s, "SHOW") && (has(s, "CREATE") || has(s, "DATABASES") || has(s, "TABLES")) {
		return true
	}
	if strings.HasPrefix(s, "DESCRIBE") {
		return true
	}
	return false
}

func runStandardSQL(slct string, db *DB) *PipeReader {
	if isQuery(slct) {
		return runQuery(slct, db)
	}
	return runExec(slct, db)
}

// query runs slct and writes the retrieved rows into pipe wr.
func query(slct string, db *DB, wr *PipeWriter) error {
	defer func(startAt time.Time) {
		log.Debugf("runQuery %v finished, elapsed:%v", slct, time.Since(startAt))
	}(time.Now())

	rows, err := db.Query(slct)
	if err != nil {
		return fmt.Errorf("runQuery failed: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %v", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return fmt.Errorf("failed to get columnTypes: %v", err)
	}

	header := make(map[string]interface{})
	header["columnNames"] = columns
	if e := wr.Write(header); e != nil {
		return e
	}

	for rows.Next() {
		if e := parseRow(columns, columnTypes, rows, wr); e != nil {
			return e
		}
	}
	return nil
}

// parseRow calls rows.Scan to retrieve the current row, and convert
// each cell value from {}interface to an accurary value.  It then
// writes the converted row into wr.
func parseRow(columns []string, columnTypes []*sql.ColumnType, rows *sql.Rows, wr *PipeWriter) error {
	// Since we don't know the table schema in advance, we create
	// a slice of empty interface and add column types at
	// runtime. Some databases support dynamic types between rows,
	// such as sqlite's affinity. So we move columnTypes inside
	// the row.Next() loop.
	count := len(columns)
	values := make([]interface{}, count)
	for i, ct := range columnTypes {
		v, e := createByType(ct.ScanType())
		if e != nil {
			return e
		}
		values[i] = v
	}

	if err := rows.Scan(values...); err != nil {
		return err
	}

	row := make([]interface{}, count)
	for i, val := range values {
		v, e := parseVal(val)
		if e != nil {
			return e
		}
		row[i] = v
	}
	if e := wr.Write(row); e != nil {
		return e
	}
	return nil
}

// runQeury creates a pipe before starting a goroutine that execute
// query, which runs slct and writes retrieved rows to a pipe.
// runQuery returns the read end of the pipe.  The caller doesn't have
// to close the pipe because the query goroutine will close it after
// data retrieval.
func runQuery(slct string, db *DB) *PipeReader {
	// FIXME(tony): how to deal with large tables?
	// TODO(tony): test on null table elements
	rd, wr := Pipe()
	go func() {
		defer wr.Close()
		if e := query(slct, db, wr); e != nil {
			log.Errorf("runQuery error:%v", e)
			if e != ErrClosedPipe {
				if err := wr.Write(e); err != nil {
					log.Errorf("runQuery error(piping):%v", err)
				}
			}
		}
	}()
	return rd
}

func runExec(slct string, db *DB) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()

		err := func() error {
			defer func(startAt time.Time) {
				log.Debugf("runEexc %v finished, elapsed:%v", slct, time.Since(startAt))
			}(time.Now())

			res, e := db.Exec(slct)
			if e != nil {
				return fmt.Errorf("runExec failed: %v", e)
			}
			affected, e := res.RowsAffected()
			if e != nil {
				return fmt.Errorf("failed to get affected row number: %v", e)
			}
			if affected > 1 {
				return wr.Write(fmt.Sprintf("%d rows affected", affected))
			}
			// gomaxcompute does not return affected rows number
			if affected < 0 {
				return wr.Write("OK")
			}
			return wr.Write(fmt.Sprintf("%d row affected", affected))
		}()
		if err != nil {
			log.Errorf("runExec error:%v", err)
			if err != ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Errorf("runExec error(piping):%v", err)
				}
			}
		}
	}()
	return rd
}

func runExtendedSQL(slct string, db *DB, modelDir string, session *pb.Session) *PipeReader {
	rd, wr := Pipe()
	go func() {
		defer wr.Close()

		err := func() error {
			defer func(startAt time.Time) {
				log.Debugf("runExtendedSQL %v finished, elapsed:%v", slct, time.Since(startAt))
			}(time.Now())
			pr, e := newParser().Parse(slct)
			if e != nil {
				return e
			}

			// NOTE: the temporary directory must be in a host directory
			// which can be mounted to Docker containers.  If I don't
			// specify the "/tmp" prefix, ioutil.TempDir would by default
			// generate a directory in /private/tmp for macOS, which
			// cannot be mounted by Docker into the container.  For more
			// detailed, please refer to
			// https://docs.docker.com/docker-for-mac/osxfs/#namespaces.
			cwd, e := ioutil.TempDir("/tmp", "sqlflow")
			if e != nil {
				return e
			}
			defer os.RemoveAll(cwd)

			if pr.train {
				if os.Getenv("SQLFLOW_submitter") == "elasticdl" {
					return elasticDLTrain(wr, pr, db, cwd, session, nil)
				}
				// TODO(weiguo): fix the hard code 0.8
				ds, e := newTrainAndValDataset(db, pr.standardSelect.String(), pr.standardSelect.tables[0], 0.8)
				if e != nil {
					return e
				}
				defer releaseTrainAndValDataset(db, ds)

				// FIXME(weiguo): temporary branch to alps
				if os.Getenv("SQLFLOW_submitter") == "alps" {
					return alpsTrain(wr, pr, db, cwd, session, ds)
				}
				return train(wr, pr, db, cwd, modelDir, slct, ds)
			}

			if pr.analyze {
				return analyze(wr, pr, db, cwd, modelDir)
			}

			// FIXME(weiguo): temporary branch to alps
			if os.Getenv("SQLFLOW_submitter") == "alps" {
				return alpsPred(wr, pr, db, cwd, session)
			} else if os.Getenv("SQLFLOW_submitter") == "elasticdl" {
				return elasticDLPredict(wr, pr, db, cwd, session, nil)
			}
			return pred(wr, pr, db, cwd, modelDir)
		}()

		if err != nil {
			log.Errorf("runExtendedSQL error:%v", err)
			if err != ErrClosedPipe {
				if err := wr.Write(err); err != nil {
					log.Errorf("runExtendedSQL error(piping):%v", err)
				}
			}
		}
	}()
	return rd
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
		log.Debugf("Train script output: %s", cw.prev)
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

func buildFiller(es *extendedSelect, ds *trainAndValDataset, fts fieldTypes, db *DB) (filler interface{}, e error) {
	// trainAndValDataset only work in train mode
	var dataset *trainAndValDataset
	if es.train {
		dataset = ds
	}
	if strings.HasPrefix(strings.ToUpper(es.estimator), `XGBOOST.`) {
		filler, e = newXGBoostFiller(es, dataset, fts, db)
		if e != nil {
			e = fmt.Errorf("failed to build XGBoostFiller: %v", e)
		}
	} else {
		filler, e = newFiller(es, dataset, fts, db)
		if e != nil {
			e = fmt.Errorf("failed to build TensorFlowFiller: %v", e)
		}
	}
	return filler, e
}

func train(wr *PipeWriter, tr *extendedSelect, db *DB, cwd string, modelDir string, slct string, ds *trainAndValDataset) error {
	fts, e := verify(tr, db)
	if e != nil {
		return e
	}

	var program bytes.Buffer
	if strings.HasPrefix(strings.ToUpper(tr.estimator), `XGBOOST.`) {
		// TODO(sperlingxx): write a separate train pipeline for xgboost to support remote mode
		if e := genXG(&program, tr, ds, fts, db); e != nil {
			return fmt.Errorf("genXG %v", e)
		}
	} else {
		if e := genTF(&program, tr, ds, fts, db); e != nil {
			return fmt.Errorf("genTF %v", e)
		}
	}

	cw := &logChanWriter{wr: wr}
	defer cw.Close()
	cmd := tensorflowCmd(cwd, db.driverName)
	cmd.Stdin = &program
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("training failed %v", e)
	}
	m := model{workDir: cwd, TrainSelect: slct}
	if modelDir != "" {
		return m.saveTar(modelDir, tr.save)
	}
	return m.save(db, tr.save)
}

func loadModelMeta(pr *extendedSelect, db *DB, cwd string, modelDir string) (*extendedSelect, fieldTypes, error) {
	var m *model
	var e error
	if modelDir != "" {
		m, e = loadTar(modelDir, cwd, pr.model)
	} else {
		m, e = load(db, pr.model, cwd)
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

func pred(wr *PipeWriter, pr *extendedSelect, db *DB, cwd string, modelDir string) error {
	pr, fts, e := loadModelMeta(pr, db, cwd, modelDir)
	if e != nil {
		return fmt.Errorf("loadModelMeta %v", e)
	}

	var buf bytes.Buffer
	if strings.HasPrefix(strings.ToUpper(pr.estimator), `XGBOOST.`) {
		// TODO(sperlingxx): write a separate pred pipeline for xgboost to support remote mode
		if e := genXG(&buf, pr, nil, fts, db); e != nil {
			return fmt.Errorf("genXG %v", e)
		}
	} else {
		if e := genTF(&buf, pr, nil, fts, db); e != nil {
			return fmt.Errorf("genTF %v", e)
		}
	}

	cw := &logChanWriter{wr: wr}
	defer cw.Close()
	cmd := tensorflowCmd(cwd, db.driverName)
	cmd.Env = append(os.Environ())
	cmd.Stdin = &buf
	cmd.Stdout = cw
	cmd.Stderr = cw
	return cmd.Run()
}

func analyze(wr *PipeWriter, es *extendedSelect, db *DB, cwd string, modelDir string) error {
	//pr, fts, e := loadModelMeta(es, db, cwd, modelDir)
	//if e != nil {
	//	return fmt.Errorf("loadModelMeta %v", e)
	//}
	//filler, e := buildFiller(pr, nil, fts, db)
	//if e != nil {
	//	return e
	//}
	//switch filler.(type) {
	//case *xgboostFiller:
	//
	//default:
	//
	//}
	cmd := exec.Command("python", "-u")
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(analyzeTemplateText)
	_, err := cmd.CombinedOutput()
	if err != nil {
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
	img2html := "<html><body><img src=\"data:image/png;base64," + imgBase64Str + "\" /></body></html>"
	wr.Write(img2html)

	return nil
}

// Create prediction table with appropriate column type.
// If prediction table already exists, it will be overwritten.
func createPredictionTable(predParsed *extendedSelect, db *DB) error {
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
	typ, _ := fts.get(predParsed.label)
	stype, e := universalizeColumnType(db.driverName, typ)
	if e != nil {
		return e
	}
	if db.driverName == "hive" {
		hdfsPath := os.Getenv("SQLFLOW_HIVE_LOCATION_ROOT_PATH")
		if hdfsPath == "" {
			hdfsPath = "/sqlflow"
		}
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE LOCATION \"%s/%s\" ;", columnName, stype, hdfsPath, tableName)
	} else {
		fmt.Fprintf(&b, "%s %s);", columnName, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func parseTableColumn(s string) (string, string, error) {
	pos := strings.LastIndex(s, ".")
	if pos == -1 || pos == len(s)-1 {
		return "", "", fmt.Errorf("can not separate %s to table and column", s)
	}
	return s[:pos], s[pos+1:], nil
}
