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
	"path"
	"sync"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var submitterRegistry = map[string](Submitter){
	"default": &defaultSubmitter{},
	"pai":     &paiSubmitter{&defaultSubmitter{}},
	"alisa":   &alisaSubmitter{},
	// TODO(typhoonzero): add submitters like alps, elasticdl
}

// SubmitterRegister registes a submitter
func SubmitterRegister(name string, submitter Submitter) {
	if submitter == nil {
		panic("submitter: Register submitter twice")
	}
	if _, dup := submitterRegistry[name]; dup {
		panic("submitter: Register called twice")
	}
	submitterRegistry[name] = submitter
}

// GetSubmitter returns a proper Submitter from configuations in environment variables.
func GetSubmitter() Submitter {
	envSubmitter := os.Getenv("SQLFLOW_submitter")
	s := submitterRegistry[envSubmitter]
	if s == nil {
		s = submitterRegistry["default"]
	}
	return s
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
	m := model{workDir: s.Cwd, TrainSelect: cl.OriginalSQL}
	modelURI := cl.Into
	if s.ModelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", s.ModelDir, cl.Into)
	}
	return m.save(modelURI, cl, s.Session)
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

func (s *defaultSubmitter) ExecuteQuery(sql *ir.StandardSQL) error {
	return runStandardSQL(s.Writer, string(*sql), s.Db)
}

func (s *defaultSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	var code string
	if isXGBoostModel(cl.Estimator) {
		code, e = xgboost.Train(cl, s.Session)
	} else {
		code, e = tensorflow.Train(cl, s.Session)
	}
	if e == nil {
		if e = s.runCommand(code); e == nil {
			e = s.SaveModel(cl)
		}
	}
	return e
}

func (s *defaultSubmitter) ExecutePredict(cl *ir.PredictStmt) (e error) {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	if e = createPredictionTableFromIR(cl, s.Db, s.Session); e == nil {
		var code string
		if isXGBoostModel(cl.TrainStmt.Estimator) {
			code, e = xgboost.Pred(cl, s.Session)
		} else {
			code, e = tensorflow.Pred(cl, s.Session)
		}
		if e == nil {
			e = s.runCommand(code)
		}
	}
	return e
}

func (s *defaultSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	var code string
	var err error
	if isXGBoostModel(cl.TrainStmt.Estimator) {
		code, err = xgboost.Explain(cl, s.Session)
	} else {
		code, err = tensorflow.Explain(cl, s.Session)
	}

	if err != nil {
		return err
	}
	if err = s.runCommand(code); err != nil {
		return err
	}
	imgFile, err := os.Open(path.Join(s.Cwd, "summary.png"))
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
	termFigure, err := ioutil.ReadFile(path.Join(s.Cwd, "summary.txt"))
	if err != nil {
		return err
	}
	s.Writer.Write(Figures{img2html, string(termFigure)})
	return nil
}

func (s *defaultSubmitter) GetTrainStmtFromModel() bool { return true }
