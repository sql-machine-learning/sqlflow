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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/cmd/repl"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func makeTestSession(dbConnStr string) *pb.Session {
	return &pb.Session{DbConnStr: dbConnStr}
}

func dummyChecker(s string) error {
	return nil
}

func trainLogChecker(s string) error {
	expectLog := "Done training"
	if strings.Contains(s, expectLog) {
		return nil
	}
	return fmt.Errorf("train sql log does not contain the expected content: %s", expectLog)
}
func TestStepTrainSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)

	sql := `SELECT * FROM iris.train WHERE class!=2
	TO TRAIN DNNClassifier
	WITH
		model.n_classes = 2,
		model.hidden_units = [10, 10],
		train.batch_size = 4,
		validation.select = "SELECT * FROM iris.test WHERE class!=2",
		validation.metrics = "Accuracy,AUC"
	LABEL class
	INTO sqlflow_models.mytest_model;`
	out, e := repl.GetStdout(func() error { return run(sql, session) })
	a.NoError(e)
	a.NoError(trainLogChecker(out))
}

func TestStepStandardSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)
	sql := `SELECT * FROM iris.train limit 5;`
	out, e := repl.GetStdout(func() error {
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
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)
	sql := `-- this is comment {a.b}
	SELECT 1, 'a';\n\t
`
	_, e := repl.GetStdout(func() error {
		return run(sql, session)
	})
	a.NoError(e)
}
