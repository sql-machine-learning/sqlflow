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
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func makeTestSession(dbConnStr string) *pb.Session {
	return &pb.Session{DbConnStr: dbConnStr}
}

func checkStepWrapper(f func(), check func(string) error) error {
	oldStdout, oldStderr := os.Stdout, os.Stderr // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	log.SetOutput(os.Stdout)
	// call the test function
	f()
	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	// back to normal state
	w.Close()
	os.Stdout = oldStdout // restoring the real stdout
	os.Stderr = oldStderr
	out := <-outC
	return check(out)
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
	a.NoError(checkStepWrapper(func() { a.NotPanics(func() { runSQLStmt(sql, session) }) }, trainLogChecker))
}

func TestStepStandardSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)
	sql := `SELECT * FROM iris.train limit 5;`
	a.NoError(checkStepWrapper(func() { a.NotPanics(func() { runSQLStmt(sql, session) }) }, func(s string) error {
		checkHead := false
		checkRows := 0
		for _, line := range strings.Split(s, "\n") {
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
		if checkHead == true && checkRows == 5 {
			return nil
		}
		return fmt.Errorf("check select result failed, checkHead: %v, checkRows: %d", checkHead, checkRows)
	}))
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
	a.NoError(checkStepWrapper(func() { a.NotPanics(func() { runSQLStmt(sql, session) }) }, dummyChecker))
}
