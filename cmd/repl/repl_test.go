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
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c-bata/go-prompt"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	"sqlflow.org/sqlflow/pkg/sql/testdata"
	"sqlflow.org/sqlflow/pkg/step"
)

var space = regexp.MustCompile(`\s+`)
var dbConnStr = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
var testDBDriver = os.Getenv("SQLFLOW_TEST_DB")
var session = sql.MakeSessionFromEnv()

func prepareTestDataOrSkip(t *testing.T) error {
	assertConnectable(dbConnStr)
	testDB, _ := database.OpenAndConnectDB(dbConnStr)
	if testDBDriver == "mysql" {
		_, e := testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
		if e != nil {
			return e
		}
		return testdata.Popularize(testDB.DB, testdata.IrisSQL)
	}
	t.Skip("Skipping mysql tests")
	return nil
}

func TestRunStmt(t *testing.T) {
	a := assert.New(t)
	a.NoError(prepareTestDataOrSkip(t))
	os.Setenv("SQLFLOW_log_dir", "/tmp/")
	session.DbConnStr = dbConnStr
	currentDB = ""
	// TODO(yancey1989): assert should not panics in repl
	output, err := step.GetStdout(func() error { return runStmt("show tables", true, "", dbConnStr) })
	a.NoError(err)
	a.Contains(output, "Error 1046: No database selected")
	output, err = step.GetStdout(func() error { return runStmt("use iris", true, "", dbConnStr) })
	a.NoError(err)
	a.Contains(output, "Database changed to iris")

	output, err = step.GetStdout(func() error { return runStmt("show tables", true, "", dbConnStr) })
	a.NoError(err)
	a.Contains(output, "| TABLES IN IRIS |")

	output, err = step.GetStdout(func() error {
		return runStmt("select * from train to train DNNClassifier WITH model.hidden_units=[10,10], model.n_classes=3, validation.select=\"select * from test\" label class INTO sqlflow_models.repl_dnn_model;", true, "", dbConnStr)
	})
	a.NoError(err)
	a.Contains(output, "'global_step': 110")

	output, err = step.GetStdout(func() error {
		return runStmt("select * from train to train xgboost.gbtree WITH objective=reg:squarederror, validation.select=\"select * from test\" label class INTO sqlflow_models.repl_xgb_model;", true, "", dbConnStr)
	})
	a.NoError(err)
	a.Contains(output, "Evaluation result: ")

	output, err = step.GetStdout(func() error {
		return runStmt("select * from train to explain sqlflow_models.repl_xgb_model;", true, "", dbConnStr)
	})
	a.NoError(err)
	a.Contains(output, "data:text/html, <div align='center'><img src='data:image/png;base64")
	a.Contains(output, "⣿") //non sixel with ascii art
}

func TestRepl(t *testing.T) {
	a := assert.New(t)
	a.Nil(prepareTestDataOrSkip(t))
	session.DbConnStr = dbConnStr
	sql := `
--
use iris; --
-- 1
show tables; -- 2
select * from train to train DNNClassifier
WITH model.hidden_units=[10,10], model.n_classes=3, validation.select="select * from test"
label class
INTO sqlflow_models.repl_dnn_model;
use sqlflow_models;
show tables`
	scanner := bufio.NewScanner(strings.NewReader(sql))
	output, err := step.GetStdout(func() error { repl(scanner, "", dbConnStr); return nil })
	a.Nil(err)
	a.Contains(output, "Database changed to iris")
	a.Contains(output, `
+----------------+
| TABLES IN IRIS |
+----------------+
| iris_empty     |
| test           |
| test_dense     |
| train          |
| train_dense    |
+----------------+`)
	a.Contains(output, `
select * from train to train DNNClassifier
WITH model.hidden_units=[10,10], model.n_classes=3, validation.select="select * from test"
label class
INTO sqlflow_models.repl_dnn_model;`)
	a.Contains(output, "'global_step': 110")
	a.Contains(output, "Database changed to sqlflow_models")
	a.Contains(output, "| TABLES IN SQLFLOW MODELS |")
	a.Contains(output, "| repl_dnn_model           |")
}

func TestMain(t *testing.T) {
	a := assert.New(t)
	a.Nil(prepareTestDataOrSkip(t))
	os.Args = append(os.Args, "-datasource", dbConnStr, "-e", "use iris; show tables", "-model_dir", "/tmp/repl_test")
	output, _ := step.GetStdout(func() error { main(); return nil })
	a.Contains(output, `
+----------------+
| TABLES IN IRIS |
+----------------+
| iris_empty     |
| test           |
| test_dense     |
| train          |
| train_dense    |
+----------------+`)
}

func testGetDataSource(t *testing.T, dataSource, databaseName string) {
	a := assert.New(t)
	a.Equal(dataSource, getDataSource(dataSource, databaseName))
	db, err := database.GetDatabaseName(dataSource)
	a.NoError(err)
	a.Equal(databaseName, db)
	a.NotEqual(dataSource, getDataSource(dataSource, databaseName+"test"))
	db, err = database.GetDatabaseName(getDataSource(dataSource, databaseName+"test"))
	a.NoError(err)
	a.Equal(databaseName+"test", db)
}
func TestGetDataSource(t *testing.T) {
	testGetDataSource(t, "maxcompute://test:test@service.cn.maxcompute.aliyun.com/api?curr_project=iris&scheme=https", "iris")
	testGetDataSource(t, "maxcompute://test:test@service.cn.maxcompute.aliyun.com/api?curr_project=&scheme=https", "")

	testGetDataSource(t, "mysql://root:root@tcp(127.0.0.1:3306)/", "")
	testGetDataSource(t, "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0", "")
	testGetDataSource(t, "mysql://root:root@tcp(127.0.0.1:3306)/iris", "iris")
	testGetDataSource(t, "mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0", "iris")

	testGetDataSource(t, "hive://root:root@localhost:10000/", "")
	testGetDataSource(t, "hive://root:root@127.0.0.1:10000/?auth=NOSASL", "")
	testGetDataSource(t, "hive://root:root@localhost:10000/churn", "churn")
	testGetDataSource(t, "hive://root:root@127.0.0.1:10000/iris?auth=NOSASL", "iris")

	b64v := base64.RawURLEncoding.EncodeToString([]byte("{\"a\":\"b\"}"))
	testGetDataSource(t, fmt.Sprintf("alisa://admin:admin@dataworks.aliyun.com?curr_project=iris&env=%s&schema=http&with=%s", b64v, b64v), "iris")
}

func testMainFastFail(t *testing.T, interactive bool) {
	a := assert.New(t)
	// Run the crashing code when FLAG is set
	if os.Getenv("SQLFLOW_TEST_REPL_FAST_FAIL_INTERACTIVE_OR_NOT") == "false" {
		os.Args = []string{os.Args[0], "--datasource", "database://in?imagination", "-e", ";"}
		main()
	} else if os.Getenv("SQLFLOW_TEST_REPL_FAST_FAIL_INTERACTIVE_OR_NOT") == "true" {
		os.Args = []string{os.Args[0], "--datasource", "database://in?imagination"}
		main()
	}
	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFastFail")
	cmd.Env = append(os.Environ(), fmt.Sprintf("SQLFLOW_TEST_REPL_FAST_FAIL_INTERACTIVE_OR_NOT=%v", interactive))
	cmd.Start()

	done := make(chan error)
	go func() { done <- cmd.Wait() }()
	timeout := time.After(4 * time.Second) // 4s are enough for **fast** fail

	select {
	case <-timeout:
		cmd.Process.Kill()
		assert.FailNowf(t, "subprocess main timed out", "interactive: %v", interactive)
	case err := <-done:
		a.Error(err)
		// Cast the error as *exec.ExitError and compare the result
		e, ok := err.(*exec.ExitError)
		expectedErrorString := "exit status 1"
		assert.Equal(t, true, ok)
		assert.Equal(t, expectedErrorString, e.Error())
	}
}

func TestMainFastFail(t *testing.T) {
	testMainFastFail(t, true)
	testMainFastFail(t, false)
}

func TestReadStmt(t *testing.T) {
	a := assert.New(t)
	sql := `SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
				model.hidden_units=[10,20],
				model.n_classes=3
			LABEL class INTO sqlflow_models.my_model;`
	scanner := bufio.NewScanner(strings.NewReader(sql))
	stmt, err := readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))

	sql2 := `-- 1. test
             SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
				model.hidden_units=[10,20],
				model.n_classes=3
             LABEL class INTO sqlflow_models.my_model;`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(0, len(stmt)) // The leading one-line comment is considered an empty statement
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))

	sql2 = `-- 1. test`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(0, len(stmt))

	sql2 = `--`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(0, len(stmt))

	sql2 = `--1. test`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Equal(io.EOF, err) // Don't support standard comment
	a.Equal(1, len(stmt))
	a.Equal(sql2, stmt[0])

	sql2 = `SHOW databases;`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))

	sql2 = `SHOW databases`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	stmt, err = readStmt(scanner)
	a.Equal(err, io.EOF) // EOF is considered the same as ';'
	a.Equal(1, len(stmt))

	sql3 := `SELECT
           *
		   FROM
		   iris.train
		   TO
		   TRAIN
		   DNNClassifier
		   WITH
		   model.hidden_units=[10,20],
		   model.n_classes=3
		   LABEL
		   class
		   INTO
		   sqlflow_models.my_model;`
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))

	sql3 = `SELECT --
           * -- comment
		   FROM -- comment;
		   iris.train -- comment ;
		   TO -- comment         ;      TRAIN
		   TRAIN
		   DNNClassifier
		   WITH
		   model.hidden_units=[10,20],
		   model.n_classes=3
		   LABEL
		   class
		   INTO
		   sqlflow_models.my_model;`
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))

	sql3 = `SELECT * FROM tbl WHERE a==";";`
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a==";\"';` // Test unclosed quote
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Equal(io.EOF, err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a==";
	        ";` // Test cross-line quoted string
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a=="\";
	        ";` // Test Escaping
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a=="';
	        ";` // Test single quote in double-quoted string
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a=='";
	        ';` // Test double quote in single-quoted string
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a=="-- \";
	        ";` // Test double dash in quoted string
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a==--" \";
	        '";` // Test quoted string in standard comment (not comment actually )
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal(stmt[0], sql3)

	sql3 = `SELECT * FROM tbl WHERE a==-- " \";
	        '";` // Test quoted string in comment, note that the quoted string is unclosed
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Equal(io.EOF, err)
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), `SELECT * FROM tbl WHERE a== '";`)

	sql3 = `--
            -- 1. test
            use iris; show
            tables; --
			select * from tbl where a not like '-- %'
	        ;` // Test multiple statements in multiple lines
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(0, len(stmt))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(0, len(stmt))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(3, len(stmt))
	a.Equal("use iris;", stmt[0])
	a.Equal("show tables;", space.ReplaceAllString(stmt[1], " "))
	a.Equal(" select * from tbl where a not like '-- %' ;", space.ReplaceAllString(stmt[2], " "))

	sql3 = `use iris; show tables;` // Test multiple statements in single line
	scanner = bufio.NewScanner(strings.NewReader(sql3))
	stmt, err = readStmt(scanner)
	a.Nil(err)
	a.Equal(2, len(stmt))
	a.Equal("use iris;", stmt[0])
	a.Equal("show tables;", space.ReplaceAllString(stmt[1], " "))

	sql4 := `SELECT\t\n1;\n\n`
	scanner = bufio.NewScanner(strings.NewReader(sql4))
	stmt, err = readStmt(scanner)
	fmt.Println(stmt)
	a.Nil(err)
	a.Equal(1, len(stmt))
	a.Equal("SELECT\t\n1;", stmt[0])

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

	var stmt []string
	a.Equal(0, len(s.statements))
	scanner := bufio.NewScanner(strings.NewReader(sql))
	for scanner.Scan() {
		s.execute(scanner.Text(), func(s string) { stmt = append(stmt, s) })
	}
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))
	a.Equal(0, len(s.statements))

	stmt = []string{}
	sql2 := `-- 1. test
SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
    model.hidden_units=[10,20],
    model.n_classes=3
LABEL class INTO sqlflow_models.my_model;`
	scanner = bufio.NewScanner(strings.NewReader(sql2))
	for scanner.Scan() {
		s.execute(scanner.Text(), func(s string) { stmt = append(stmt, s) })
	}
	a.Equal(1, len(stmt))
	a.Equal(space.ReplaceAllString(stmt[0], " "), space.ReplaceAllString(sql, " "))
}

func TestInputNavigation(t *testing.T) {
	attribute.ExtractDocStringsOnce()
	a := assert.New(t)
	s := newPromptState()
	his1 := "history 1"
	his2 := "history 2"
	his3 := "SELECT * FROM iris.tran WHERE class like '%中文';"
	s.history = []prompt.Suggest{{his3, ""}, {his2, ""}, {his1, ""}}
	p := prompt.NewBuffer()
	// put something on input buffer
	p.InsertText(his3, false, true)
	// go backward
	s.navigateHistory("", true, p)
	a.Equal(his3, p.Text())
	s.navigateHistory("", true, p)
	a.Equal(his2, p.Text())
	s.navigateHistory("", true, p)
	a.Equal(his1, p.Text())
	// should stop at last history
	s.navigateHistory("", true, p)
	a.Equal(his1, p.Text())
	s.navigateHistory("", true, p)
	a.Equal(his1, p.Text())
	// go forward
	s.navigateHistory("", false, p)
	a.Equal(his2, p.Text())
	s.navigateHistory("", false, p)
	a.Equal(his3, p.Text())
	s.navigateHistory("", false, p)
	a.Equal("", p.Text())
}

func TestComplete(t *testing.T) {
	attribute.ExtractDocStringsOnce()
	a := assert.New(t)
	s := newPromptState()
	p := prompt.NewBuffer()
	// Imitating the `input from console` process
	p.InsertText(`SELECT * FROM iris.train T`, false, true)
	c := s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("TO", c[0].Text)

	p.InsertText(`O T`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("TRAIN", c[0].Text)

	p.InsertText(`RAIN `, false, true)
	c = s.completer(*p.Document())
	a.Equal(18, len(c))
	a.Equal("BoostedTreesClassifier", c[0].Text)

	p.InsertText(`DNN`, false, true)
	c = s.completer(*p.Document())
	a.Equal(4, len(c))

	p.InsertText(`c`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("DNNClassifier", c[0].Text)
	p.DeleteBeforeCursor(1) // TODO(shendiaomo): It's sort of case sensitive at the moment

	p.InsertText(`C`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("DNNClassifier", c[0].Text)

	p.InsertText(`lassifier w`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("WITH", c[0].Text)

	p.InsertText(`ith `, false, true)
	c = s.completer(*p.Document())
	a.Equal(20, len(c))

	p.InsertText(`model.f`, false, true)
	c = s.completer(*p.Document())
	a.Equal(0, len(c)) // model.feature_columns removed by codegen/attribute.go
	p.DeleteBeforeCursor(1)

	p.InsertText(`h`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("model.hidden_units", c[0].Text)

	p.InsertText(`idden_units=[400,300], `, false, true)
	c = s.completer(*p.Document())
	a.Equal(20, len(c))

	p.InsertText(`o`, false, true)
	c = s.completer(*p.Document())
	a.Equal(5, len(c)) // Adagrad has 5 parameters
	p.DeleteBeforeCursor(1)

	p.InsertText(`model.optimizer=`, false, true)
	c = s.completer(*p.Document())
	a.Equal(8, len(c))
	a.Equal("Adadelta", c[0].Text)

	p.InsertText(`R`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("RMSprop", c[0].Text)

	p.InsertText(`MSprop,`, false, true)
	c = s.completer(*p.Document())
	p.InsertText(` o`, false, true) // FIXME(shendiaomo): copy-n-paste doesn't work here
	c = s.completer(*p.Document())
	a.Equal(7, len(c)) // RMSprop has 7 parameters
	a.Equal("optimizer", c[0].Text)

	p.InsertText(`ptimizer.learning_rate=0.02, model.n`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("model.n_classes", c[0].Text)

	p.InsertText(`_classes=3 l`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("LABEL", c[0].Text)

	p.InsertText(`abel class i`, false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("INTO", c[0].Text)

	p.InsertText(`nto `, false, true)
	c = s.completer(*p.Document())
	a.Equal(0, len(c))

	p.InsertText(`nto sqlflow_models.my_awesome_model;`, false, true)
	c = s.completer(*p.Document())
	a.Equal(0, len(c))

	// Test cross line completion
	s = newPromptState()
	s.statements = []string{"TO"}
	p = prompt.NewBuffer()
	p.InsertText("t", false, true)
	c = s.completer(*p.Document())
	a.Equal(1, len(c))
	a.Equal("TRAIN", c[0].Text)
}

func TestTerminalCheck(t *testing.T) {
	a := assert.New(t)
	_, err := exec.LookPath("it2check")
	a.Nil(err)
	a.False(it2Check)
}

func TestGetTerminalColumnSize(t *testing.T) {
	a := assert.New(t)
	a.Equal(1024, getTerminalColumnSize())
	oldConsoleParser := consoleParser
	consoleParser = newTestConsoleParser()
	a.Equal(238, getTerminalColumnSize())
	consoleParser = oldConsoleParser
}

func applyEmacsMetaKeyBinding(buf *prompt.Buffer, key []byte) {
	for _, binding := range emacsMetaKeyBindings {
		if bytes.Compare(binding.ASCIICode, key) == 0 {
			binding.Fn(buf)
		}
	}
}

func applyEmacsControlKeyBinding(buf *prompt.Buffer, key prompt.Key) {
	for _, binding := range emacsCtrlKeyBindings {
		if binding.Key == key {
			binding.Fn(buf)
		}
	}
}

func TestEmacsKeyBindings(t *testing.T) {
	a := assert.New(t)
	buf := prompt.NewBuffer()
	buf.InsertText("USE iris", false, true)
	a.Equal(8, buf.DisplayCursorPosition())

	applyEmacsControlKeyBinding(buf, prompt.ControlA)
	a.Equal(0, buf.DisplayCursorPosition())

	applyEmacsControlKeyBinding(buf, prompt.ControlE)
	a.Equal(8, buf.DisplayCursorPosition())

	applyEmacsControlKeyBinding(buf, prompt.ControlB)
	a.Equal(7, buf.DisplayCursorPosition())
	a.Equal("iri", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("s", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlF)
	a.Equal(8, buf.DisplayCursorPosition())

	applyEmacsControlKeyBinding(buf, prompt.ControlH) // Delete the character before cursor ('s')
	a.Equal(7, buf.DisplayCursorPosition())
	a.Equal("iri", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlW) // Cut the word before cursor ('iri') to the clipboard
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('iri') back
	a.Equal(7, buf.DisplayCursorPosition())
	a.Equal("iri", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlB) // Move back a character (between 'ir' and 'i')
	applyEmacsControlKeyBinding(buf, prompt.ControlK) // Cut the line after the cursor to the clipboard ('i')
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('i') back
	a.Equal(7, buf.DisplayCursorPosition())
	a.Equal("iri", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlU) // Cut the line before the cursor to the clipboard ('USE iri')
	a.Equal(0, buf.DisplayCursorPosition())
	a.Equal("", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('USE iri') back
	a.Equal(7, buf.DisplayCursorPosition())
	a.Equal("iri", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlB) // Move back a character (between 'ir' and 'i')
	applyEmacsControlKeyBinding(buf, prompt.ControlD) // Delete the word under the cursor ('i')
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'b'}) // Move cursor left by a word (at 'i')
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("ir", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'f'}) // Move back
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'B'}) // Meta B/F
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("ir", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'F'})
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 0x1b, 0x5b, 0x44}) // Meta <-/->
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("ir", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 0x1b, 0x5b, 0x43})
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 0x7f}) // Cut the word before cursor ('ir') to the clipboard
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('ir') back
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'b'}) // Move cursor left by a word (at 'i')
	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'd'}) // Cut the word after cursor ('ir') to the clipboard
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('ir') back
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'b'}) // Move cursor left by a word (at 'i')
	applyEmacsMetaKeyBinding(buf, []byte{0x1b, 'D'}) // Cut the word after cursor ('ir') to the clipboard
	a.Equal(4, buf.DisplayCursorPosition())
	a.Equal("USE ", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlY) // Paste ('ir') back
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())

	applyEmacsControlKeyBinding(buf, prompt.ControlL) // cls
	a.Equal(6, buf.DisplayCursorPosition())
	a.Equal("ir", buf.Document().GetWordBeforeCursorWithSpace())
	a.Equal("", buf.Document().GetWordAfterCursorWithSpace())
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
func (p *testConsoleParser) GetWinSize() *prompt.WinSize { return &prompt.WinSize{73, 238} }
