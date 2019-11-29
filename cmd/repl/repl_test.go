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
	"container/list"
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
	var testdb *sf.DB
	var err error
	switch os.Getenv("SQLFLOW_TEST_DB") {
	case "mysql":
		dataSourceStr = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
		testdb, err = sf.NewDB(dataSourceStr)
		a.NoError(err)
		defer testdb.Close()
		err = testdata.Popularize(testdb.DB, testdata.IrisSQL)
		a.NoError(err)
	case "hive":
		dataSourceStr = "hive://root:root@127.0.0.1:10000/iris?auth=NOSASL"
		testdb, err = sf.NewDB(dataSourceStr)
		a.NoError(err)
		defer testdb.Close()
		err = testdata.Popularize(testdb.DB, testdata.IrisHiveSQL)
		a.NoError(err)
	default:
		t.Skipf("skip TestStdinParseOnly for db type: %s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	os.Setenv("SQLFLOW_DATASOURCE", dataSourceStr)
	var stdin bytes.Buffer
	trainSQL := `SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20], train.batch_size = 10, train.epoch = 2
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.mymodel;`
	stdin.Write([]byte(trainSQL))
	pbtxt, err := parseSQLFromStdin(&stdin)
	a.NoError(err)
	pbTrain := &irpb.TrainClause{}
	proto.UnmarshalText(pbtxt, pbTrain)
	a.Equal("class", pbTrain.GetLabel().GetNc().GetFieldMeta().GetName())

	// run one train SQL to save the model then test predict/analyze use the model
	sess := &irpb.Session{DbConnStr: dataSourceStr}
	stream := sf.RunSQLProgram(trainSQL, testdb, "", sess)
	lastResp := list.New()
	keepSize := 10
	for rsp := range stream.ReadAll() {
		switch rsp.(type) {
		case error:
			var s []string
			for e := lastResp.Front(); e != nil; e = e.Next() {
				s = append(s, e.Value.(string))
			}
			a.Fail(strings.Join(s, "\n"))
		}
		lastResp.PushBack(rsp)
		if lastResp.Len() > keepSize {
			e := lastResp.Front()
			lastResp.Remove(e)
		}
	}

	stdin.Reset()
	stdin.Write([]byte("SELECT * from iris.train TO PREDICT iris.predict.class USING sqlflow_models.mymodel;"))
	pbtxt, err = parseSQLFromStdin(&stdin)
	a.NoError(err)
	pbPred := &irpb.PredictClause{}
	proto.UnmarshalText(pbtxt, pbPred)
	a.Equal("class", pbPred.GetResultColumn())

	stdin.Reset()
	stdin.Write([]byte(`SELECT * from iris.train TO EXPLAIN sqlflow_models.mymodel WITH shap_summary.plot_type="bar" USING TreeExplainer;`))
	pbtxt, err = parseSQLFromStdin(&stdin)
	a.NoError(err)
	pbAnalyze := &irpb.AnalyzeClause{}
	proto.UnmarshalText(pbtxt, pbAnalyze)
	a.Equal("TreeExplainer", pbAnalyze.GetExplainer())
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
