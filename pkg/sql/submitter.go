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
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/model"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
)

// GetSubmitter returns a proper Submitter from configurations in environment variables.
func GetSubmitter(submitter string) Submitter {
	if submitter == "" {
		submitter = os.Getenv("SQLFLOW_submitter")
	}
	switch submitter {
	case "default":
		return &defaultSubmitter{}
	case "pai":
		return &paiSubmitter{&defaultSubmitter{}}
	case "alisa":
		return &alisaSubmitter{&defaultSubmitter{}}
	// TODO(typhoonzero): add submitters like alps, elasticdl
	default:
		return &defaultSubmitter{}
	}
}

// Figures contains analyzed figures as strings
type Figures struct {
	Image string
	Text  string
}

// Submitter extends ir.Executor
type Submitter interface {
	ir.Executor
	Setup(*pipe.Writer, *database.DB, string, string, *pb.Session)
	GetTrainStmtFromModel() bool
}

type logChanWriter struct {
	wr   *pipe.Writer
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

type defaultSubmitter struct {
	Writer   *pipe.Writer
	Db       *database.DB
	ModelDir string
	Cwd      string
	Session  *pb.Session
}

func (s *defaultSubmitter) Setup(w *pipe.Writer, db *database.DB, modelDir string, cwd string, session *pb.Session) {
	// cwd is used to store train scripts and save output models.
	s.Writer, s.Db, s.ModelDir, s.Cwd, s.Session = w, db, modelDir, cwd, session
}

func (s *defaultSubmitter) SaveModel(cl *ir.TrainStmt) error {
	m := model.New(s.Cwd, cl.OriginalSQL)
	modelURI := cl.Into
	if s.ModelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", s.ModelDir, cl.Into)
	}
	return m.Save(modelURI, cl, s.Session)
}

func (s *defaultSubmitter) runCommand(program string) error {
	cw := &logChanWriter{wr: s.Writer}
	var output bytes.Buffer
	w := io.MultiWriter(cw, &output)
	defer cw.Close()
	cmd := sqlflowCmd(s.Cwd, s.Db.DriverName)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(program), w, w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("failed: %v\n%sProgram%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", program, output.String())
	}
	return nil
}

func (s *defaultSubmitter) ExecuteQuery(sql *ir.NormalStmt) error {
	return runNormalStmt(s.Writer, string(*sql), s.Db)
}

func (s *defaultSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	var code string
	if isXGBoostModel(cl.Estimator) {
		if code, e = xgboost.Train(cl, s.Session); e != nil {
			return e
		}
	} else {
		if code, e = tensorflow.Train(cl, s.Session); e != nil {
			return e
		}
	}
	if e := s.runCommand(code); e != nil {
		return e
	}
	return s.SaveModel(cl)
}

func (s *defaultSubmitter) ExecutePredict(cl *ir.PredictStmt) (e error) {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	if e = createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}

	var code string
	if isXGBoostModel(cl.TrainStmt.Estimator) {
		if code, e = xgboost.Pred(cl, s.Session); e != nil {
			return e
		}
	} else {
		if code, e = tensorflow.Pred(cl, s.Session); e != nil {
			return e
		}
	}
	return s.runCommand(code)
}

func (s *defaultSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	var code string
	var err error
	db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	if isXGBoostModel(cl.TrainStmt.Estimator) {
		code, err = xgboost.Explain(cl, s.Session)
		// TODO(typhoonzero): deal with XGBoost model explain result table creation.
	} else {
		code, err = tensorflow.Explain(cl, s.Session)
		if cl.Into != "" {
			err := createExplainResultTable(db, cl, cl.Into, pai.ModelTypeTF, cl.TrainStmt.Estimator)
			if err != nil {
				return err
			}
		}
	}

	if err != nil {
		return err
	}
	if err = s.runCommand(code); err != nil {
		return err
	}
	img, err := readExplainResult(path.Join(s.Cwd, "summary.png"))
	if err != nil {
		return err
	}
	termFigure, err := ioutil.ReadFile(path.Join(s.Cwd, "summary.txt"))
	if err != nil {
		return err
	}
	s.Writer.Write(Figures{img, string(termFigure)})
	return nil
}

func (s *defaultSubmitter) ExecuteEvaluate(cl *ir.EvaluateStmt) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	var code string
	var err error
	if isXGBoostModel(cl.TrainStmt.Estimator) {
		code, err = xgboost.Evaluate(cl, s.Session)
		if err != nil {
			return err
		}
	} else {
		code, err = tensorflow.Evaluate(cl, s.Session)
		if err != nil {
			return err
		}
	}

	if cl.Into != "" {
		// create evaluation result table
		db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
		if err != nil {
			return err
		}
		// default always output evaluation loss
		metricNames := []string{"loss"}
		metricsAttr, ok := cl.Attributes["validation.metrics"]
		if ok {
			metricsList := strings.Split(metricsAttr.(string), ",")
			metricNames = append(metricNames, metricsList...)
		}
		err = createEvaluationResultTable(db, cl.Into, metricNames)
		if err != nil {
			return err
		}
	}
	if err = s.runCommand(code); err != nil {
		return err
	}
	return nil
}

func createEvaluationResultTable(db *database.DB, tableName string, metricNames []string) error {
	dropStmt := fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName)
	var e error
	if _, e = db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	columnDef := ""
	columnDefList := []string{}
	if db.DriverName == "mysql" {
		for _, mn := range metricNames {
			columnDefList = append(columnDefList,
				fmt.Sprintf("%s VARCHAR(255)", mn))
		}

	} else {
		// Hive, MaxCompute
		for _, mn := range metricNames {
			columnDefList = append(columnDefList,
				fmt.Sprintf("%s STRING", mn))
		}
	}
	columnDef = strings.Join(columnDefList, ",")
	createStmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s);`, tableName, columnDef)
	if _, e = db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func readExplainResult(target string) (string, error) {
	r, err := os.Open(target)
	if err != nil {
		return "", err
	}
	defer r.Close()
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	img := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("<div align='center'><img src='data:image/png;base64,%s' /></div>", img), nil
}

func (s *defaultSubmitter) GetTrainStmtFromModel() bool { return true }

func (s *defaultSubmitter) ExecuteShowTrain(showTrain *ir.ShowTrainStmt) error {
	model, err := model.Load(showTrain.ModelName, "", s.Db)
	if err != nil {
		s.Writer.Write("Load model meta " + showTrain.ModelName + " failed.")
		return err
	}
	header := make(map[string]interface{})
	header["columnNames"] = []string{"Table", "Train Statement"}
	s.Writer.Write(header)
	s.Writer.Write([]interface{}{showTrain.ModelName, strings.TrimSpace(model.TrainSelect)})

	return nil
}
