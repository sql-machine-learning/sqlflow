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
	"reflect"
	"sqlflow.org/sqlflow/pkg/database"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	pyts "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"

	sfargo "sqlflow.org/sqlflow/pkg/argo"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	sf "sqlflow.org/sqlflow/pkg/sql"
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
	return sfargo.Fetch(job)
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
			res, err = encodeHead(s)
		case []interface{}:
			res, err = encodeRow(s)
		case sf.Figures:
			res, err = encodeMessage(s.Image)
		case string:
			res, err = encodeMessage(s)
		case sf.WorkflowJob:
			job := r.(sf.WorkflowJob)
			res = &pb.Response{Response: &pb.Response_Job{Job: &pb.Job{Id: job.JobID}}}
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

func encodeHead(head map[string]interface{}) (*pb.Response, error) {
	cn, ok := head["columnNames"]
	if !ok {
		return nil, fmt.Errorf("can't find field columnNames in head")
	}
	columnNames, ok := cn.([]string)
	if !ok {
		return nil, fmt.Errorf("head[\"columnNames\"] is of type %T, expected []string", cn)
	}
	return &pb.Response{Response: &pb.Response_Head{Head: &pb.Head{ColumnNames: columnNames}}}, nil
}

func encodeRow(row []interface{}) (*pb.Response, error) {
	encodedRow := &pb.Row{}
	for _, element := range row {
		pm, err := encodePODType(element)
		if err != nil {
			return nil, err
		}
		any, err := ptypes.MarshalAny(pm)
		if err != nil {
			return nil, err
		}
		encodedRow.Data = append(encodedRow.Data, any)
	}
	return &pb.Response{Response: &pb.Response_Row{Row: encodedRow}}, nil
}

func encodeMessage(s string) (*pb.Response, error) {
	return &pb.Response{Response: &pb.Response_Message{Message: &pb.Message{Message: s}}}, nil
}

func encodePODType(val interface{}) (proto.Message, error) {
	switch v := val.(type) {
	case nil:
		return &pb.Row_Null{}, nil
	case bool:
		return &wrappers.BoolValue{Value: v}, nil
	case int8, int16, int32:
		return &wrappers.Int32Value{Value: int32(reflect.ValueOf(val).Int())}, nil
	case int, int64:
		return &wrappers.Int64Value{Value: int64(reflect.ValueOf(val).Int())}, nil
	case uint8, uint16, uint32:
		return &wrappers.UInt32Value{Value: uint32(reflect.ValueOf(val).Uint())}, nil
	case uint, uint64:
		return &wrappers.UInt64Value{Value: uint64(reflect.ValueOf(val).Uint())}, nil
	case float32:
		return &wrappers.FloatValue{Value: v}, nil
	case float64:
		return &wrappers.DoubleValue{Value: v}, nil
	case string:
		return &wrappers.StringValue{Value: v}, nil
	case []byte:
		return &wrappers.BytesValue{Value: v}, nil
	case time.Time:
		return &pyts.Timestamp{
			Seconds: int64(v.Unix()),
			Nanos:   int32(v.Nanosecond())}, nil
	default:
		return nil, fmt.Errorf("Unknown Go type %#v to be converted into protobuf.Any", val)
	}
}
