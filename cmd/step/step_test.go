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
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/step"
)

func makeTestSession(dbConnStr string) *pb.Session {
	return &pb.Session{DbConnStr: dbConnStr}
}

func TestStepStandardSQL(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skip("skip no mysql test.")
	}
	a := assert.New(t)
	dbConnStr := "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0"
	session := makeTestSession(dbConnStr)
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
func TestImage(t *testing.T) {
	a := assert.New(t)
	a.True(isHTMLSnippet("<div></div>"))
	_, err = getBase64EncodedImage("")
	a.Error(err)
	image, err := getBase64EncodedImage(testImageHTML)
	a.Nil(err)
	a.Nil(imageCat(image)) // sixel mode
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
	_, e := step.GetStdout(func() error {
		return run(sql, session)
	})
	a.NoError(e)
}
