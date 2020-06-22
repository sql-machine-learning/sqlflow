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
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"sqlflow.org/sqlflow/pkg/codegen/optimize"

	"sqlflow.org/sqlflow/pkg/codegen/pai"
	"sqlflow.org/sqlflow/pkg/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/model"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

var rePyDiagnosis = regexp.MustCompile("sqlflow_submitter.tensorflow.diag.SQLFlowDiagnostic: (.*)")

// Figures contains analyzed figures as strings
type Figures struct {
	Image string
	Text  string
}

// New returns a proper Submitter from configurations in environment variables.
func New(executor string) ir.Executor {
	if executor == "" {
		executor = os.Getenv("SQLFLOW_submitter")
	}
	switch executor {
	case "default":
		return &pythonExecutor{}
	case "pai":
		return &paiExecutor{&pythonExecutor{}}
	case "alisa":
		return &alisaExecutor{&pythonExecutor{}}
	// TODO(typhoonzero): add executor like alps, elasticdl
	default:
		return &pythonExecutor{}
	}
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

type pythonExecutor struct {
	Writer   *pipe.Writer
	Db       *database.DB
	ModelDir string
	Cwd      string
	Session  *pb.Session
}

func (s *pythonExecutor) Setup(w *pipe.Writer, db *database.DB, modelDir string, cwd string, session *pb.Session) {
	// cwd is used to store train scripts and save output models.
	s.Writer, s.Db, s.ModelDir, s.Cwd, s.Session = w, db, modelDir, cwd, session
}

func (s *pythonExecutor) SaveModel(cl *ir.TrainStmt) error {
	m := model.New(s.Cwd, cl.OriginalSQL)
	modelURI := cl.Into
	if s.ModelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", s.ModelDir, cl.Into)
	}
	return m.Save(modelURI, s.Session)
}

func (s *pythonExecutor) runCommand(program string, logStderr bool) error {
	cw := &logChanWriter{wr: s.Writer}
	defer cw.Close()
	cmd := sqlflowCmd(s.Cwd, s.Db.DriverName)
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	if logStderr {
		w := io.MultiWriter(cw, &stderr)
		wStdout := bufio.NewWriter(&stdout)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(program), wStdout, w
	} else {
		w := io.MultiWriter(cw, &stdout)
		wStderr := bufio.NewWriter(&stderr)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(program), w, wStderr
	}
	if e := cmd.Run(); e != nil {
		// return the diagnostic message
		sub := rePyDiagnosis.FindStringSubmatch(stderr.String())
		if len(sub) == 2 {
			return fmt.Errorf("%s", sub[1])
		}
		// if no diagnostic message, return the full stack trace
		return fmt.Errorf("failed: %v\n%sGenerated Code:%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", program, stderr.String())
	}
	return nil
}

func (s *pythonExecutor) ExecuteQuery(stmt *ir.NormalStmt) error {
	return runNormalStmt(s.Writer, string(*stmt), s.Db)
}

func (s *pythonExecutor) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	var code string
	if cl.GetModelKind() == ir.XGBoost {
		if code, e = xgboost.Train(cl, s.Session); e != nil {
			return e
		}
	} else {
		if code, e = tensorflow.Train(cl, s.Session); e != nil {
			return e
		}
	}
	if e := s.runCommand(code, false); e != nil {
		return e
	}
	return s.SaveModel(cl)
}

func (s *pythonExecutor) ExecutePredict(cl *ir.PredictStmt) (e error) {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	if e = createPredictionResultTable(cl, s.Db, s.Session); e != nil {
		return e
	}

	var code string
	if cl.TrainStmt.GetModelKind() == ir.XGBoost {
		if code, e = xgboost.Pred(cl, s.Session); e != nil {
			return e
		}
	} else {
		if code, e = tensorflow.Pred(cl, s.Session); e != nil {
			return e
		}
	}
	return s.runCommand(code, false)
}

func (s *pythonExecutor) ExecuteExplain(cl *ir.ExplainStmt) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	var code string
	var err error
	db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()
	if cl.TrainStmt.GetModelKind() == ir.XGBoost {
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
	if err = s.runCommand(code, false); err != nil {
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

func (s *pythonExecutor) ExecuteEvaluate(cl *ir.EvaluateStmt) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	var code string
	var err error
	if cl.TrainStmt.GetModelKind() == ir.XGBoost {
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
		defer db.Close()
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
	if err = s.runCommand(code, false); err != nil {
		return err
	}
	return nil
}

func generateOptFlowOptimizeCodeAndExecute(cl *ir.OptimizeStmt, submitter *pythonExecutor, session *pb.Session, cwd string, dbName string, tableName string, isPai bool) error {
	// Generate optimization code
	runnerFileName := "custom_optimize_runner"
	runnerCode, submitCode, err := optimize.GenerateOptFlowOptimizeCode(cl, session, dbName, tableName,
		runnerFileName)

	if err != nil {
		return err
	}

	// Write the runner code to cwd for submission
	runnerFilePath := fmt.Sprintf("%s/%s.py", cwd, runnerFileName)
	err = ioutil.WriteFile(runnerFilePath, []byte(runnerCode), 0644)
	if err != nil {
		return err
	}

	if isPai {
		err = copyPythonPackage("sqlflow_submitter", cwd)
		if err != nil {
			return err
		}
	}

	// Note: OptFlow submit API logs on stderr but not stdout
	if err = submitter.runCommand(submitCode, true); err != nil {
		return err
	}
	return nil
}

func (s *pythonExecutor) ExecuteOptimize(cl *ir.OptimizeStmt) error {
	// TODO(sneaxiy): to be implemented
	return fmt.Errorf("ExecuteOptimize is not supported in default submitter")
}

func (s *pythonExecutor) ExecuteRun(runStmt *ir.RunStmt) error {
	// TODO(brightcoder01): Add the implementation in the following PR.
	return fmt.Errorf("ExecuteRun is not implemeneted in default executor yet")
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

func (s *pythonExecutor) GetTrainStmtFromModel() bool { return true }

func (s *pythonExecutor) ExecuteShowTrain(showTrain *ir.ShowTrainStmt) error {
	model, err := model.Load(showTrain.ModelName, "", s.Db)
	if err != nil {
		s.Writer.Write("Load model meta " + showTrain.ModelName + " failed.")
		return err
	}
	header := make(map[string]interface{})
	header["columnNames"] = []string{"Model", "Train Statement"}
	s.Writer.Write(header)
	s.Writer.Write([]interface{}{showTrain.ModelName, strings.TrimSpace(model.TrainSelect)})

	return nil
}
