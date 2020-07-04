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

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/step"
)

func makeTestSession(dbConnStr string) *pb.Session {
	return &pb.Session{DbConnStr: dbConnStr}
}

func TestStepStandardSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	session := makeTestSession(database.GetTestingMySQLURL())
	sql := `SELECT * FROM iris.train limit 5;`
	out, e := step.GetStdout(func() error {
		return run(sql, session)
	})
	// check output result
	a.NoError(e)
	checkHead := false
	checkRows := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		response := &pb.Response{}
		if e := proto.UnmarshalText(line, response); e == nil {
			if response.GetHead() != nil {
				checkHead = true
			} else if response.GetRow() != nil {
				checkRows++
			} else {
				continue
			}
		}
	}
	a.True(checkHead)
	a.Equal(checkRows, 5)
}

func TestStepSQLWithComment(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	session := makeTestSession(database.GetTestingMySQLURL())
	sql := `-- this is comment {a.b}
	SELECT 1, 'a';\n\t
`
	_, e := step.GetStdout(func() error {
		return run(sql, session)
	})
	a.NoError(e)
}
