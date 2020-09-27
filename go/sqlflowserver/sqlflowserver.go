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
	"time"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/workflow"

	"github.com/golang/protobuf/proto"
	submitter "sqlflow.org/sqlflow/go/executor"
	"sqlflow.org/sqlflow/go/parser"
	"sqlflow.org/sqlflow/go/pipe"
	pb "sqlflow.org/sqlflow/go/proto"
	sf "sqlflow.org/sqlflow/go/sql"
)

// Server is the instance will be used to connect to DB and execute training
type Server struct {
	// TODO(typhoonzero): should pass `Server` struct to run function, so that we can get
	// server-side configurations together with client side session in the run context.
	// To do this we need to refactor current pkg structure, so that we will not have circular dependency.
	run      func(sql string, modelDir string, session *pb.Session) *pipe.Reader
	modelDir string
}

// NewServer returns a server instance
func NewServer(run func(string, string, *pb.Session) *pipe.Reader,
	modelDir string) *Server {
	return &Server{run: run, modelDir: modelDir}
}

// Fetch implements `rpc Fetch (Job) returns(JobStatus)`
func (s *Server) Fetch(ctx context.Context, job *pb.FetchRequest) (*pb.FetchResponse, error) {
	// FIXME(tony): to make function fetch easily to mock, we should decouple server package
	// with argo package by introducing s.fetch
	_, wf, e := workflow.New(getWorkflowBackend())
	if e != nil {
		return nil, e
	}
	return wf.Fetch(job)
}

// Run implements `rpc Run (Request) returns (stream Response)`
func (s *Server) Run(req *pb.Request, stream pb.SQLFlow_RunServer) error {
	rd := s.run(req.Sql, s.modelDir, req.Session)
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
			sqls, err := parser.Parse(dialect, req.Sql)
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
func SubmitWorkflow(sqlProgram string, modelDir string, session *pb.Session) *pipe.Reader {
	logger := log.WithFields(log.Fields{
		"requestID": log.UUID(),
		"user":      session.UserId,
		"submitter": session.Submitter,
		"event":     "submitWorkflow",
	})
	if os.Getenv("SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT") == "" {
		logger.Fatalf("should set SQLFLOW_WORKFLOW_LOGVIEW_ENDPOINT if enable argo mode.")
	}
	rd, wr := pipe.Pipe()
	startTime := time.Now()
	go func() {
		defer wr.Close()
		wfID, e := workflow.Run(getWorkflowBackend(), sqlProgram, session, logger)
		defer logger.Infof("submitted, workflowID:%s, spent:%.f, SQL:%s, error:%v", wfID, time.Since(startTime).Seconds(), sqlProgram, e)
		if e != nil {
			if e := wr.Write(e); e != nil {
				logger.Errorf("piping: %v", e)
			}
			return
		}
		if e := wr.Write(pb.Job{Id: wfID}); e != nil {
			logger.Errorf("piping: %v", e)
			return
		}
	}()
	return rd
}

func getWorkflowBackend() string {
	wfBackend := os.Getenv("SQLFLOW_WORKFLOW_BACKEND")
	if wfBackend == "" {
		wfBackend = "couler"
	}
	return wfBackend
}
