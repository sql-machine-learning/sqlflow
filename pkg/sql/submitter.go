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

	"sqlflow.org/sqlflow/pkg/pipe"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var envSubmitter = os.Getenv("SQLFLOW_submitter")

var submitterRegistry = map[string](Submitter){
	"default": &defaultSubmitter{},
	"pai":     &paiSubmitter{&defaultSubmitter{}},
	// TODO(typhoonzero): add submitters like alps, elasticdl
}

// Submitter is a visitor that generates and executes code for SQLStatement
type Submitter interface {
	ExecuteQuery(*ir.StandardSQL, *RequestContext, string) error
	ExecuteTrain(*ir.TrainStmt, *RequestContext, string) error
	ExecutePredict(*ir.PredictStmt, *RequestContext, string) error
	ExecuteAnalyze(*ir.AnalyzeStmt, *RequestContext, string) error
	GetTrainStmtFromModel() bool
}

func getSubmitter() Submitter {
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

type defaultSubmitter struct{}

func (s *defaultSubmitter) SaveModel(cl *ir.TrainStmt, req *RequestContext, cwd string) error {
	m := model{workDir: cwd, TrainSelect: cl.OriginalSQL}
	modelURI := cl.Into
	if req.ModelSaveDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", req.ModelSaveDir, cl.Into)
	}
	return m.save(modelURI, cl, req.Session)
}

func (s *defaultSubmitter) runCommand(program string, req *RequestContext, cwd string) error {
	cw := &logChanWriter{wr: req.Wr}
	var output bytes.Buffer
	w := io.MultiWriter(cw, &output)
	defer cw.Close()
	cmd := sqlflowCmd(cwd, req.Conn.DriverName)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(program), w, w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("failed: %v\n%sProgram%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", program, output.String())
	}
	return nil
}

func (s *defaultSubmitter) ExecuteQuery(sql *ir.StandardSQL, req *RequestContext, cwd string) error {
	return runStandardSQL(req.Wr, string(*sql), req.Conn)
}

func (s *defaultSubmitter) ExecuteTrain(cl *ir.TrainStmt, req *RequestContext, cwd string) (e error) {
	var code string
	if isXGBoostModel(cl.Estimator) {
		code, e = xgboost.Train(cl)
	} else {
		code, e = tensorflow.Train(cl)
	}
	if e == nil {
		if e = s.runCommand(code, req, cwd); e == nil {
			e = s.SaveModel(cl, req, cwd)
		}
	}
	return e
}

func (s *defaultSubmitter) ExecutePredict(cl *ir.PredictStmt, req *RequestContext, cwd string) (e error) {
	// NOTE(typhoonzero): model is already loaded under cwd when generating ir.PredictStmt
	if e = createPredictionTableFromIR(cl, req.Conn, req.Session); e == nil {
		var code string
		if isXGBoostModel(cl.TrainStmt.Estimator) {
			code, e = xgboost.Pred(cl, req.Session)
		} else {
			code, e = tensorflow.Pred(cl, req.Session)
		}
		if e == nil {
			e = s.runCommand(code, req, cwd)
		}
	}
	return e
}

func (s *defaultSubmitter) ExecuteAnalyze(cl *ir.AnalyzeStmt, req *RequestContext, cwd string) error {
	// NOTE(typhoonzero): model is already loaded under s.Cwd
	if !isXGBoostModel(cl.TrainStmt.Estimator) {
		return fmt.Errorf("unsupported model %s", cl.TrainStmt.Estimator)
	}

	code, err := xgboost.Analyze(cl)
	if err != nil {
		return err
	}
	if err = s.runCommand(code, req, cwd); err != nil {
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
	termFigure, err := ioutil.ReadFile(path.Join(cwd, "summary.txt"))
	if err != nil {
		return err
	}
	req.Wr.Write(Figures{img2html, string(termFigure)})
	return nil
}

func (s *defaultSubmitter) GetTrainStmtFromModel() bool { return true }
