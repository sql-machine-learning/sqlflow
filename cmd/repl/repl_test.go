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

package main

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/stretchr/testify/assert"

	prompt "github.com/c-bata/go-prompt"
	irpb "sqlflow.org/sqlflow/pkg/proto"
	sf "sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/sql/testdata"
)

// TODO(shendiaomo): end to end tests like sqlflowserver/main_test.go

var space = regexp.MustCompile(`\s+`)

func TestReadStmt(t *testing.T) {
	a := assert.New(t)
	sql := `SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
				model.hidden_units=[10,20],
				model.n_classes=3
			LABEL class INTO sqlflow_models.my_model;`
	scanner := bufio.NewScanner(strings.NewReader(sql))
	stmt, err := readStmt(scanner)
	a.Nil(err)
	a.Equal(space.ReplaceAllString(stmt, " "), space.ReplaceAllString(sql, " "))
}

func TestPromptState(t *testing.T) {
	a := assert.New(t)
	s := newPromptState()
	a.Equal(len(s.keywords), 15)
	sql := `SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
				model.hidden_units=[10,20],
				model.n_classes=3
			LABEL class INTO sqlflow_models.my_model;`
	words := strings.Fields(sql)
	keyword, ahead, last := s.lookaheadKeyword(words)
	a.Equal("INTO", keyword)
	a.Equal("class", ahead)
	a.Equal("sqlflow_models.my_model;", last)

	keyword, ahead, last = s.lookaheadKeyword(words[0 : len(words)-1])
	a.Equal("INTO", keyword)
	a.Equal("class", ahead)
	a.Equal("INTO", last)

	keyword, ahead, last = s.lookaheadKeyword(words[0 : len(words)-2])
	a.Equal("LABEL", keyword)
	a.Equal("model.n_classes=3", ahead)
	a.Equal("class", last)

	keyword, ahead, last = s.lookaheadKeyword(words[0 : len(words)-4])
	a.Equal("WITH", keyword)
	a.Equal("DNNClassifier", ahead)
	a.Equal("model.n_classes=3", last)

	var stmt string
	scanner := bufio.NewScanner(strings.NewReader(sql))
	for scanner.Scan() {
		s.execute(scanner.Text(), func(s string) { stmt = s })
	}
	a.Equal(space.ReplaceAllString(stmt, " "), space.ReplaceAllString(sql, " "))
	a.Equal("", s.statement)
}

func TestStdinParser(t *testing.T) {
	a := assert.New(t)
	p := newTestConsoleParser()
	buf, e := p.Read()
	a.Nil(e)
	a.Equal("test multiple", string(buf))

	buf, e = p.Read()
	a.Nil(e)
	a.Equal(prompt.Enter, prompt.GetKey(buf))

	buf, e = p.Read()
	a.Nil(e)
	a.Equal("line paste", strings.TrimSpace(string(buf)))

	buf, e = p.Read()
	a.Nil(e)
	a.Equal("test multiple", string(buf))
}

func TestStdinParseOnly(t *testing.T) {
	a := assert.New(t)
	dataSourceStr := ""
	switch os.Getenv("SQLFLOW_TEST_DB") {
	case "mysql":
		dataSourceStr = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
		testdb, err := sf.NewDB(dataSourceStr)
		a.NoError(err)
		defer testdb.Close()
		err = testdata.Popularize(testdb.DB, testdata.IrisSQL)
		a.NoError(err)
	case "hive":
		dataSourceStr = "hive://root:root@127.0.0.1:10000/iris?auth=NOSASL"
		testdb, err := sf.NewDB(dataSourceStr)
		a.NoError(err)
		defer testdb.Close()
		err = testdata.Popularize(testdb.DB, testdata.IrisHiveSQL)
		a.NoError(err)
	default:
		t.Skipf("skip TestStdinParseOnly for db type: %s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	os.Setenv("SQLFLOW_DATASOURCE", dataSourceStr)
	var stdin bytes.Buffer
	stdin.Write([]byte("SELECT * from iris.train TO TRAIN DNNClassifier WITH a=1 LABEL class INTO mymodel;"))
	pbtxt, err := parseSQLFromStdin(&stdin)
	a.NoError(err)
	pbIRToTest := &irpb.TrainIR{}
	proto.UnmarshalText(pbtxt, pbIRToTest)
	a.Equal("class", pbIRToTest.GetLabel().GetNc().GetFieldMeta().GetName())
}

type testConsoleParser struct{}

func (p *testConsoleParser) Read() ([]byte, error) {
	input := `test multiple
				line paste`
	return []byte(input), nil
}

// newStdinParser returns ConsoleParser object to read from stdin.
func newTestConsoleParser() *stdinParser {
	return &stdinParser{
		ConsoleParser: &testConsoleParser{},
	}
}

func (p *testConsoleParser) Setup() error                { return nil }
func (p *testConsoleParser) TearDown() error             { return nil }
func (p *testConsoleParser) GetWinSize() *prompt.WinSize { return nil }
