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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sqlflow.org/sqlflow/go/codegen/experimental"
	"strings"
	"sync"

	"sqlflow.org/sqlflow/go/verifier"

	"sqlflow.org/sqlflow/go/codegen/optimize"

	"sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/codegen/xgboost"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/model"
	"sqlflow.org/sqlflow/go/pipe"
	pb "sqlflow.org/sqlflow/go/proto"
)

var rePyDiagnostics = regexp.MustCompile("runtime.diagnostics.SQLFlowDiagnostic: (.*)")

const (
	sqlflowToRunContextKeySelect = "SQLFLOW_TO_RUN_SELECT"
	sqlflowToRunContextKeyInto   = "SQLFLOW_TO_RUN_INTO"
	sqlflowToRunContextKeyImage  = "SQLFLOW_TO_RUN_IMAGE"
	sqlflowToRunProgramFolder    = "/opt/sqlflow/run"
)

// Figures contains analyzed figures as strings
type Figures struct {
	Image string
	Text  string
}

// Executor call code geneartor to generate submitter program and execute it.
type Executor interface {
	Setup(*pipe.Writer, *database.DB, string, string, *pb.Session)
	ExecuteQuery(*ir.NormalStmt) error
	ExecuteTrain(*ir.TrainStmt) error
	ExecutePredict(*ir.PredictStmt) error
	ExecuteExplain(*ir.ExplainStmt) error
	ExecuteEvaluate(*ir.EvaluateStmt) error
	ExecuteShowTrain(*ir.ShowTrainStmt) error
	ExecuteOptimize(*ir.OptimizeStmt) error
	ExecuteRun(*ir.RunStmt) error
	GetTrainStmtFromModel() bool
}

// New returns a proper Submitter from configurations in environment variables.
func New(executor string) Executor {
	if executor == "" {
		executor = os.Getenv("SQLFLOW_submitter")
	}
	switch executor {
	case "default", "local":
		return &pythonExecutor{}
	case "pai":
		return &paiExecutor{&pythonExecutor{}}
	case "pai_local":
		return &paiLocalExecutor{&pythonExecutor{}}
	case "alisa":
		return &alisaExecutor{&pythonExecutor{}}
	case "alps":
		return &alpsExecutor{&pythonExecutor{}}
	// TODO(typhoonzero): add executor for elasticdl
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

// Run interprets the SQLFlow IR.
// TODO(yancey1989): this is a temporary way to decouple executor from the ir package,
// as the discussion of https://github.com/sql-machine-learning/sqlflow/issues/2494,
// SQLFlow would generate target code instead of interpret an IR.
func Run(it Executor, stmt ir.SQLFlowStmt) error {
	switch v := stmt.(type) {
	case *ir.TrainStmt:
		return it.ExecuteTrain(stmt.(*ir.TrainStmt))
	case *ir.PredictStmt:
		return it.ExecutePredict(stmt.(*ir.PredictStmt))
	case *ir.ExplainStmt:
		return it.ExecuteExplain(stmt.(*ir.ExplainStmt))
	case *ir.EvaluateStmt:
		return it.ExecuteEvaluate(stmt.(*ir.EvaluateStmt))
	case *ir.OptimizeStmt:
		return it.ExecuteOptimize(stmt.(*ir.OptimizeStmt))
	case *ir.RunStmt:
		return it.ExecuteRun(stmt.(*ir.RunStmt))
	case *ir.NormalStmt:
		return it.ExecuteQuery(stmt.(*ir.NormalStmt))
	case *ir.ShowTrainStmt:
		return it.ExecuteShowTrain(stmt.(*ir.ShowTrainStmt))
	default:
		return fmt.Errorf("unregistered SQLFlow IR type: %s", v)
	}
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

func useExperimentalExecutor(exec Executor) (bool, error) {
	if os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") != "true" {
		return false, nil
	}

	if pyExec, ok := exec.(*pythonExecutor); ok {
		dialect, _, err := database.ParseURL(pyExec.Session.DbConnStr)
		if err != nil {
			return false, err
		}

		// TODO(sneaxiy): remove this line when PyAlisa is ready.
		if dialect == "alisa" {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

func (s *pythonExecutor) tryExperimentalExecute(sqlStmt ir.SQLFlowStmt, logStderr bool) (bool, error) {
	ok, err := useExperimentalExecutor(s)
	if err != nil {
		return true, err
	}
	if !ok {
		return false, nil
	}

	// NOTE(sneaxiy): should use the image here
	stepCode, _, err := experimental.GenerateStepCodeAndImage(sqlStmt, 0, s.Session, nil)
	stepFuncCode, err := experimental.GetPyFuncBody(stepCode, "step_entry_0")
	if err != nil {
		return true, err
	}

	const bashCodeTmpl = `python <<EOF
%s
EOF
`

	cmd := exec.Command("bash", "-c", fmt.Sprintf(bashCodeTmpl, stepFuncCode))
	cmd.Dir = s.Cwd
	errorLog, err := s.runCommand(cmd, nil, logStderr)
	if err != nil {
		return true, fmt.Errorf("%v\n%s", err, errorLog)
	}
	return true, nil
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

func (s *pythonExecutor) runProgram(program string, logStderr bool) error {
	cmd := sqlflowCmd(s.Cwd, s.Db.DriverName)
	cmd.Stdin = bytes.NewBufferString(program)

	errorLog, e := s.runCommand(cmd, nil, logStderr)
	if e != nil {
		// return the diagnostic message
		sub := rePyDiagnostics.FindStringSubmatch(errorLog)
		if len(sub) == 2 {
			return fmt.Errorf("%s", sub[1])
		}
		// if no diagnostic message, return the full stack trace
		return fmt.Errorf("failed: %v\n%sGenerated Code:%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", program, errorLog)
	}
	return nil
}

func (s *pythonExecutor) runCommand(cmd *exec.Cmd, context map[string]string, logStderr bool) (string, error) {
	cw := &logChanWriter{wr: s.Writer}
	defer cw.Close()

	for k, v := range context {
		os.Setenv(k, v)
	}

	var stderr bytes.Buffer
	var stdout bytes.Buffer
	if logStderr {
		w := io.MultiWriter(cw, &stderr)
		wStdout := bufio.NewWriter(&stdout)
		cmd.Stdout, cmd.Stderr = wStdout, w
	} else {
		w := io.MultiWriter(cw, &stdout)
		wStderr := bufio.NewWriter(&stderr)
		cmd.Stdout, cmd.Stderr = w, wStderr
	}

	if e := cmd.Run(); e != nil {
		return stderr.String(), e
	}

	return ``, nil
}

func (s *pythonExecutor) ExecuteQuery(stmt *ir.NormalStmt) error {
	if ok, err := s.tryExperimentalExecute(stmt, false); ok {
		return err
	}
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
	if e := s.runProgram(code, false); e != nil {
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
	return s.runProgram(code, false)
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

	var modelType int
	if cl.TrainStmt.GetModelKind() == ir.XGBoost {
		code, err = xgboost.Explain(cl, s.Session)
		modelType = model.XGBOOST
	} else {
		code, err = tensorflow.Explain(cl, s.Session)
		modelType = model.TENSORFLOW
	}

	if cl.Into != "" {
		err := createExplainResultTable(db, cl, cl.Into, modelType, cl.TrainStmt.Estimator)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}
	if err = s.runProgram(code, false); err != nil {
		return err
	}
	if cl.Into == "" {
		img, err := readExplainResult(path.Join(s.Cwd, "summary.png"))
		if err != nil {
			return err
		}
		termFigure, err := ioutil.ReadFile(path.Join(s.Cwd, "summary.txt"))
		if err != nil {
			return err
		}
		s.Writer.Write(Figures{img, string(termFigure)})
	}
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
	if err = s.runProgram(code, false); err != nil {
		return err
	}
	return nil
}

func (s *pythonExecutor) ExecuteOptimize(stmt *ir.OptimizeStmt) error {
	db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, _, err := database.ParseURL(s.Session.DbConnStr)
	if err != nil {
		return err
	}

	fieldTypes, err := verifier.Verify(stmt.Select, db)
	if err != nil {
		return err
	}

	tableColumnsStrList := make([]string, 0)
	for name, typ := range fieldTypes {
		isVariable := false
		for _, v := range stmt.Variables {
			if strings.EqualFold(name, v) {
				isVariable = true
				break
			}
		}

		if !isVariable {
			continue
		}

		typ, err = fieldType(driver, typ)
		if err != nil {
			return err
		}
		tableColumnsStrList = append(tableColumnsStrList, fmt.Sprintf("%s %s", name, typ))
	}

	resultColumnName := stmt.ResultValueName
	if len(stmt.Variables) == 1 && strings.EqualFold(stmt.Variables[0], resultColumnName) {
		resultColumnName += "_value"
	}

	resultColumnType := ""
	if stmt.VariableType == "Binary" || strings.HasSuffix(stmt.VariableType, "Integers") {
		resultColumnType = "BIGINT"
	} else if strings.HasSuffix(stmt.VariableType, "Reals") {
		resultColumnType = "FLOAT"
	} else {
		return fmt.Errorf("unsupported variable_type = %s", stmt.VariableType)
	}

	resultColumnType, err = fieldType(driver, resultColumnType)
	if err != nil {
		return err
	}

	tableColumnsStrList = append(tableColumnsStrList, fmt.Sprintf("%s %s", resultColumnName, resultColumnType))

	dropTmpTables([]string{stmt.ResultTable}, s.Session.DbConnStr)
	createTableSQL := fmt.Sprintf("CREATE TABLE %s (%s);", stmt.ResultTable, strings.Join(tableColumnsStrList, ","))
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	program, err := optimize.GenerateOptimizeCode(stmt, s.Session, "", false)
	if err != nil {
		return err
	}
	if err := s.runProgram(program, false); err != nil {
		return err
	}
	return nil
}

func (s *pythonExecutor) ExecuteRun(runStmt *ir.RunStmt) error {
	if len(runStmt.Parameters) == 0 {
		return fmt.Errorf("Parameters shouldn't be empty")
	}

	context := map[string]string{
		sqlflowToRunContextKeySelect: runStmt.Select,
		sqlflowToRunContextKeyInto:   runStmt.Into,
		sqlflowToRunContextKeyImage:  runStmt.ImageName,
	}

	var e error
	var errMsg string
	// The first parameter is the program name
	program := runStmt.Parameters[0]
	fileExtension := filepath.Ext(program)
	if len(fileExtension) == 0 {
		// If the file extension is empty, it's an executable binary.
		// Build the command
		cmd := exec.Command(program, runStmt.Parameters[1:]...)
		cmd.Dir = s.Cwd

		errMsg, e = s.runCommand(cmd, context, false)
	} else if strings.EqualFold(fileExtension, ".py") {
		// If the first parameter is a Python program
		// Build the command
		moduleName := strings.TrimSuffix(program, fileExtension)
		pyCmdParams := append([]string{"-m", moduleName}, runStmt.Parameters[1:]...)
		cmd := exec.Command("python", pyCmdParams...)
		cmd.Dir = s.Cwd

		errMsg, e = s.runCommand(cmd, context, false)
	} else {
		// TODO(brightcoder01): Implement the execution of the program built using other script languages.
		return fmt.Errorf("The other executable except Python program is not supported yet")
	}

	if e != nil {
		s.Writer.Write(errMsg)
	}

	return e
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
