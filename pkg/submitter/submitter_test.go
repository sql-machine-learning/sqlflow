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
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"

	"testing"
)

type TestSubmitter struct{}

func (s *TestSubmitter) ExecuteQuery(cl *ir.StandardSQL) error   { return nil }
func (s *TestSubmitter) ExecuteTrain(cl *ir.TrainStmt) error     { return nil }
func (s *TestSubmitter) ExecutePredict(cl *ir.PredictStmt) error { return nil }
func (s *TestSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error { return nil }
func (s *TestSubmitter) GetTrainStmtFromModel() bool             { return true }
func (s *TestSubmitter) setup(w *pipe.Writer, db *database.DB, modelDir string, cwd string, session *pb.Session) {
}

func TestRegister(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		Register("test", &TestSubmitter{})
	})

	// panic if register nil or register same submitter twice.
	a.Panics(func() {
		Register("test", nil)
	})
	a.Panics(func() {
		Register("test", &TestSubmitter{})
	})

	expected := []string{"test"}
	a.Equal(expected, Submitters())

	s, err := New("test", nil, nil, "", "", nil)
	a.NoError(err)

	_, ok := s.(*TestSubmitter)
	a.True(ok)
}
