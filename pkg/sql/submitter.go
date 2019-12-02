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

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/codegen/tensorflow"
	"sqlflow.org/sqlflow/pkg/sql/codegen/xgboost"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

var envSubmitter = os.Getenv("SQLFLOW_submitter")

var submitterRegistry = map[string](Submitter){
	"default":   &defaultSubmitter{},
	"alps":      &alpsSubmitter{&defaultSubmitter{}},
	"elasticdl": &elasticdlSubmitter{&defaultSubmitter{}},
}

func submitter() Submitter {
	s := submitterRegistry[envSubmitter]
	if s == nil {
		s = submitterRegistry["default"]
	}
	return s
}

// Submitter extends ir.Executor
type Submitter interface {
	ir.Executor
	Setup(*PipeWriter, *DB, string, *pb.Session) error
	Teardown()
	GetTrainIRFromModel() bool
}

type logChanWriter struct {
	wr   *PipeWriter
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
	Writer   *PipeWriter
	Db       *DB
	ModelDir string
	Cwd      string
	Session  *pb.Session
}

type elasticdlSubmitter struct{ *defaultSubmitter }
type alpsSubmitter struct{ *defaultSubmitter }

func (s *defaultSubmitter) Setup(w *PipeWriter, db *DB, modelDir string, session *pb.Session) error {
	// cwd is used to store train scripts and save output models.
	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	s.Writer, s.Db, s.ModelDir, s.Cwd, s.Session = w, db, modelDir, cwd, session
	return err
}

func (s *defaultSubmitter) SaveModel(cl *ir.TrainClause) error {
	m := model{workDir: s.Cwd, TrainSelect: cl.OriginalSQL}
	modelURI := cl.Into
	if s.ModelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", s.ModelDir, cl.Into)
	}
	return m.save(modelURI, cl, s.Session)
}

func (s *defaultSubmitter) LoadModel(cl *ir.TrainClause) error {
	modelURI := cl.Into
	if s.ModelDir != "" {
		modelURI = fmt.Sprintf("file://%s/%s", s.ModelDir, cl.Into)
	}
	_, err := load(modelURI, s.Cwd, s.Db)
	return err
}

func (s *defaultSubmitter) runCommand(program string) error {
	cw := &logChanWriter{wr: s.Writer}
	var output bytes.Buffer
	w := io.MultiWriter(cw, &output)
	defer cw.Close()
	cmd := sqlflowCmd(s.Cwd, s.Db.driverName)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(program), w, w
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("failed: %v\n%sProgram%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v", e, "==========", program, output.String())
	}
	return nil
}

func (s *defaultSubmitter) ExecuteQuery(sql *ir.StandardSQL) error {
	return runStandardSQL(s.Writer, string(*sql), s.Db)
}

func (s *defaultSubmitter) ExecuteTrain(cl *ir.TrainClause) (e error) {
	var code string
	if isXGBoostModel(cl.Estimator) {
		code, e = xgboost.Train(cl)
	} else {
		code, e = tensorflow.Train(cl)
	}
	if e == nil {
		if e = s.runCommand(code); e == nil {
			e = s.SaveModel(cl)
		}
	}
	return e
}

func (s *defaultSubmitter) ExecutePredict(cl *ir.PredictClause) (e error) {
	if e = s.LoadModel(cl.TrainIR); e == nil {
		if e = createPredictionTableFromIR(cl, s.Db, s.Session); e == nil {
			var code string
			if isXGBoostModel(cl.TrainIR.Estimator) {
				code, e = xgboost.Pred(cl, s.Session)
			} else {
				code, e = tensorflow.Pred(cl, s.Session)
			}
			if e == nil {
				e = s.runCommand(code)
			}
		}
	}
	return e
}
func (s *defaultSubmitter) ExecuteAnalyze(cl *ir.AnalyzeClause) error {
	if err := s.LoadModel(cl.TrainIR); err != nil {
		return err
	}
	if !isXGBoostModel(cl.TrainIR.Estimator) {
		return fmt.Errorf("unsupported model %s", cl.TrainIR.Estimator)
	}

	code, err := xgboost.Analyze(cl)
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
	s.Writer.Write(img2html)
	return nil
}
func (s *defaultSubmitter) Teardown()                 { os.RemoveAll(s.Cwd) }
func (s *defaultSubmitter) GetTrainIRFromModel() bool { return true }

func (s *elasticdlSubmitter) ExecuteTrain(cl *ir.TrainClause) (e error) {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	if pr, e := newExtendedSyntaxParser().Parse(cl.OriginalSQL); e == nil {
		e = elasticDLTrain(s.Writer, pr, s.Db, s.Cwd, s.Session)
	}
	return e
}

func (s *elasticdlSubmitter) ExecutePredict(cl *ir.PredictClause) (e error) {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	if pr, e := newExtendedSyntaxParser().Parse(cl.OriginalSQL); e == nil {
		e = elasticDLPredict(s.Writer, pr, s.Db, s.Cwd, s.Session)
	}
	return e
}

func (s *alpsSubmitter) ExecuteTrain(cl *ir.TrainClause) (e error) {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	if pr, e := newExtendedSyntaxParser().Parse(cl.OriginalSQL); e == nil {
		e = alpsTrain(s.Writer, pr, s.Db, s.Cwd, s.Session)
	}
	return e
}

func (s *alpsSubmitter) ExecutePredict(cl *ir.PredictClause) (e error) {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	if pr, e := newExtendedSyntaxParser().Parse(cl.OriginalSQL); e == nil {
		e = alpsPred(s.Writer, pr, s.Db, s.Cwd, s.Session)
	}
	return e
}
