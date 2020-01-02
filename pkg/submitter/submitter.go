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

package submitter

import (
	"fmt"
	"sort"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	"sqlflow.org/sqlflow/pkg/sql/ir"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

var submitterRegistry = make(map[string]Submitter)

// Submitter submites the Python program by code generator: `pkg/sql/codegen`
type Submitter interface {
	ir.Executor
	setup(*pipe.Writer, *database.DB, string, string, *pb.Session)
	GetTrainStmtFromModel() bool
}

// New returns a registered submitter implementation
func New(name string, w *pipe.Writer, db *database.DB, modelDir, cwd string, session *pb.Session) (Submitter, error) {
	submitter, ok := submitterRegistry[name]
	if !ok {
		return nil, fmt.Errorf("submittter: %s has not been registered", name)
	}
	submitter.setup(w, db, modelDir, cwd, session)
	return submitter, nil
}

// Register makes a submitter available by the providing name,
// if the submitter has already registered or registered twice,
// it panics.
func Register(name string, submitter Submitter) {
	if submitter == nil {
		panic("submitter: Register submitter twice")
	}
	if _, dup := submitterRegistry[name]; dup {
		panic("submitter: Register called twice")
	}
	submitterRegistry[name] = submitter
}

// Submitters returens a list of the registered submitter name
func Submitters() []string {
	list := []string{}
	for name := range submitterRegistry {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}
