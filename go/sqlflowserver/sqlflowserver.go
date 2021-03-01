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

// Package server is the SQLFlow grpc server which connects to database and
// parse, submit or execute the training and predicting codes.
//
// To generate grpc protobuf code, run the below command:
package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/workflow"
	"sqlflow.org/sqlflow/go/workflow/argo"

	"github.com/golang/protobuf/proto"
	submitter "sqlflow.org/sqlflow/go/executor"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/pipe"
	pb "sqlflow.org/sqlflow/go/proto"
	sf "sqlflow.org/sqlflow/go/sql"
)

// Server is the instance will be used to connect to DB and execute training
type Server struct {
	run func(sql string, session *pb.Session) *pipe.Reader
}

// NewServer returns a server instance
func NewServer(run func(string, *pb.Session) *pipe.Reader) *Server {
	return &Server{run: run}
}

// Fetch implements `rpc Fetch (Job) returns(JobStatus)`
func (s *Server) Fetch(ctx context.Context, job *pb.FetchRequest) (*pb.FetchResponse, error) {
	// FIXME(tony): to make function fetch easily to mock, we should decouple server package
	// with argo package by introducing s.fetch
	return argo.Fetch(job)
}

// Run implements `rpc Run (Request) returns (stream Response)`
func (s *Server) Run(req *pb.Request, stream pb.SQLFlow_RunServer) error {
	rd := s.run(req.Stmts, req.Session)
	defer rd.Close()

	for r := range rd.ReadAll() {
		var res *pb.Response
		var err error
		switch s := r.(type) {
		case error:
			return s
		case map[string]interface{}:
			res, err = pb.EncodeHead(s)
		case []interface{}:
			res, err = pb.EncodeRow(s)
		case submitter.Figures:
			res, err = pb.EncodeMessage(s.Image)
		case string:
			res = &pb.Response{}
			err = proto.UnmarshalText(s, res)
			if err != nil {
				res, err = pb.EncodeMessage(s)
			}
		case pb.Job:
			res = &pb.Response{Response: &pb.Response_Job{Job: &s}}
		case sf.EndOfExecution:
			// FIXME(tony): decouple server package with sql package by introducing s.numberOfStatement
			dialect, _, err := database.ParseURL(req.Session.DbConnStr)
			if err != nil {
				return err
			}
			sqls, err := parser.Parse(dialect, req.Stmts)
			if err != nil {
				return err
			}
			// if sqlStatements have only one field, do **NOT** return EndOfExecution message.
			if len(sqls) > 1 {
				eoeMsg := r.(sf.EndOfExecution)
				eoe := &pb.EndOfExecution{
					Sql:              eoeMsg.Statement,
					SpentTimeSeconds: eoeMsg.EndTime - eoeMsg.StartTime,
				}
				eoeResponse := &pb.Response{Response: &pb.Response_Eoe{Eoe: eoe}}
				if err := stream.Send(eoeResponse); err != nil {
					return err
				}
			} else {
				continue
			}
		default:
			return fmt.Errorf("unrecognized run channel return type %#v", s)
		}
		if err != nil {
			return err
		}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}

// SubmitWorkflow submits an Argo workflow
//
// TODO(wangkuiyi): Make SubmitWorkflow return an error in addition to
// *pipe.Reader, and remove the calls to log.Printf.
func SubmitWorkflow(sqlProgram string, session *pb.Session) *pipe.Reader {
	logger := log.WithFields(log.Fields{
		"requestID": log.UUID(),
		"user":      session.UserId,
		"submitter": session.Submitter,
		"event":     "submitWorkflow",
	})
	rd, wr := pipe.Pipe()

	if os.Getenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT") == "" {
		logger.Fatalf("should set SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT if enable argo mode.")
	}
	useExperimentalCodegen := os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") == "true"
	useCoulerSubmitter := os.Getenv("SQLFLOW_USE_COULER_SUBMITTER") == "true"

	startTime := time.Now()
	go func() {
		defer wr.Close()
		var yaml string
		var err error
		if !useExperimentalCodegen {
			yaml, err = workflow.CompileToYAML(sqlProgram, session, logger)
			if err != nil {
				logger.Printf("compile error: %v", err)
				if e := wr.Write(err); e != nil {
					logger.Errorf("piping error: %v", e)
				}
				return
			}
		} else if useCoulerSubmitter {
			var pycode string
			pycode, err = workflow.CompileToCoulerSubmitCode(sqlProgram, session, logger)
			if err != nil {
				logger.Printf("compile error: %v", err)
				if e := wr.Write(err); e != nil {
					logger.Errorf("piping error: %v", e)
				}
				return
			}

			cmd := exec.Command("python")
			cmd.Env = append(os.Environ())
			cmd.Stdin = strings.NewReader(pycode)
			out, err := cmd.CombinedOutput()
			if err != nil {
				logger.Printf("run couler program to submit: %v", err)
				if e := wr.Write(err); e != nil {
					logger.Errorf("piping error: %v", e)
				}
				return
			}
			if err = wr.Write(out); err != nil {
				logger.Errorf("piping error: %v", err)
			}
			// end submit here
			return
		} else {
			yaml, err = workflow.CompileToYAMLExperimental(sqlProgram, session)
			if err != nil {
				logger.Printf("compile error: %v", err)
				if e := wr.Write(err); e != nil {
					logger.Errorf("piping error: %v", e)
				}
				return
			}
		}

		wfID, e := argo.Submit(yaml)
		defer logger.Infof("submitted, workflowID:%s, spent:%.f, SQL:%s, error:%v", wfID, time.Since(startTime).Seconds(), sqlProgram, e)
		if e != nil {
			if e := wr.Write(e); e != nil {
				logger.Errorf("piping error: %v", e)
			}
			return
		}
		if e := wr.Write(pb.Job{Id: wfID}); e != nil {
			logger.Errorf("piping error: %v", e)
			return
		}
	}()
	return rd
}
