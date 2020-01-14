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

package step

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

func mockSession() *pb.Session {
	return &pb.Session{DbConnStr: "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"}
}

func mockSQLProgramIR() ir.SQLProgram {
	standardSQL := ir.StandardSQL("SELECT * FROM iris.train limit 10;")
	trainStmt := ir.MockTrainStmt(false)
	return []ir.SQLStatement{&standardSQL, trainStmt}
}

func TestRunFromSQLProgram(t *testing.T) {
	a := assert.New(t)
	cg, err := GetCodegen("couler")
	spIR := mockSQLProgramIR()
	a.NoError(err)
	_, err = RunFromSQLProgram(cg, spIR, mockSession())
	a.NoError(err)
}
