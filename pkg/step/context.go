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

package step

import (
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql"
)

// Context records all resources that each "step" in the workflow may require
type Context struct {
	SQLProgram string
	Session    *pb.Session
	// Directory for save/load model, empty if we are going to save model to db
	ModelSaveDir string
	// Pipe for write back results when processing current request
	Wr *pipe.Writer
	Rd *pipe.Reader
	// Connection to the database for each request, will be closed when request is closed.
	Conn *database.DB
	// current working directory, will generate python code, save model, load model to this path
	// Cwd string
	// Will set to true if model/checkpoint is loaded into Cwd
	// IsModelLoaded bool
	// a proper submitter for current request, e.g. Tensorflow/PAI/Elasticdl
	Submitter sql.Submitter
}

// NewRequestContext construct a new RequestContext object.
func NewRequestContext(sqlProgram string, session *pb.Session, modelSaveDir string) (*Context, error) {
	submitter := sql.GetSubmitter()
	conn, err := database.OpenAndConnectDB(session.GetDbConnStr())
	if err != nil {
		return nil, err
	}
	rd, wr := pipe.Pipe()
	req := &Context{
		SQLProgram:   sqlProgram,
		Session:      session,
		ModelSaveDir: modelSaveDir,
		Wr:           wr,
		Rd:           rd,
		Conn:         conn,
		Submitter:    submitter,
	}
	return req, nil
}

// Close release all resources of the current request.
func (req *Context) Close() error {
	// Wr should be closed when the writing is finished
	req.Rd.Close()
	if err := req.Conn.Close(); err != nil {
		return err
	}
	return nil
}
