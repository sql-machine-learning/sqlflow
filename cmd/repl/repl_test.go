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

func getStdout(f func() error) (out string, e error) {
	oldStdout, oldStderr := os.Stdout, os.Stderr // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	e = f() // f prints to stdout
	outC := make(chan string)
	go func() { // copy the output in a separate goroutine so printing can't block indefinitely
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	w.Close()                                   // Cancel redirection
	os.Stdout, os.Stderr = oldStdout, oldStderr // restoring the real stdout and stderr
	out = <-outC
	return
}

func TestRunStmt(t *testing.T) {
	a := assert.New(t)
	a.Nil(prepareTestDataOrSkip(t))
	os.Setenv("SQLFLOW_log_dir", "/tmp/")
	session.DbConnStr = dbConnStr
	currentDB = ""
	output, err := getStdout(func() error { return runStmt("show tables", true, "", dbConnStr) })
	a.Nil(err)
	a.Contains(output, "Error 1046: No database selected")

	output, err = getStdout(func() error { return runStmt("use iris", true, "", dbConnStr) })
	a.Nil(err)
	a.Contains(output, "Database changed to iris")

	output, err = getStdout(func() error { return runStmt("show tables", true, "", dbConnStr) })
	a.Nil(err)
	a.Contains(output, "| TABLES IN IRIS |")

	output, err = getStdout(func() error {
		return runStmt("select * from train to train DNNClassifier WITH model.hidden_units=[10,10], model.n_classes=3 label class INTO sqlflow_models.repl_dnn_model;", true, "", dbConnStr)
	})
	a.Nil(err)
	a.Contains(output, "'global_step': 110")

	output, err = getStdout(func() error {
		return runStmt("select * from train to train xgboost.gbtree WITH objective=reg:squarederror label class INTO sqlflow_models.repl_xgb_model;", true, "", dbConnStr)
	})
	a.Nil(err)
	a.Contains(output, "Evaluation result: ")

	output, err = getStdout(func() error {
		return runStmt("select * from train to explain sqlflow_models.repl_xgb_model;", true, "", dbConnStr)
	})
	a.Nil(err)
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
WITH model.hidden_units=[10,10], model.n_classes=3
label class
INTO sqlflow_models.repl_dnn_model;
use sqlflow_models;
show tables`
	scanner := bufio.NewScanner(strings.NewReader(sql))
	output, err := getStdout(func() error { repl(scanner, "", dbConnStr); return nil })
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
WITH model.hidden_units=[10,10], model.n_classes=3
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
	output, _ := getStdout(func() error { main(); return nil })
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
	a.Equal(databaseName, getDatabaseName(dataSource))
	a.NotEqual(dataSource, getDataSource(dataSource, databaseName+"test"))
	a.Equal(databaseName+"test", getDatabaseName(getDataSource(dataSource, databaseName+"test")))
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
	p.InsertText(` o`, false, true) // FIXME(shendiaomo): copy-n-paste doen't work here
	c = s.completer(*p.Document())
	a.Equal(7, len(c)) // RMSprop has 7 parameters
	a.Equal("optimizer", c[0].Text)

	p.InsertText(`ptimizer.learing_rate=0.02, model.n`, false, true)
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

	a.True(isHTMLSnippet("<div></div>"))
	_, err = getBase64EncodedImage("")
	a.Error(err)
	image, err := getBase64EncodedImage(testImageHTML)
	a.Nil(err)
	a.Nil(imageCat(image)) // sixel mode
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

var testImageHTML string = "<div align='center'><img src='data:image/png;base64,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L3RpZmYvMS4wLyIKICAgICAgICAgICAgeG1sbnM6ZXhpZj0iaHR0cDovL25zLmFkb2JlLmNvbS9leGlmLzEuMC8iPgogICAgICAgICA8dGlmZjpSZXNvbHV0aW9uVW5pdD4yPC90aWZmOlJlc29sdXRpb25Vbml0PgogICAgICAgICA8dGlmZjpPcmllbnRhdGlvbj4xPC90aWZmOk9yaWVudGF0aW9uPgogICAgICAgICA8dGlmZjpDb21wcmVzc2lvbj4xPC90aWZmOkNvbXByZXNzaW9uPgogICAgICAgICA8dGlmZjpQaG90b21ldHJpY0ludGVycHJldGF0aW9uPjI8L3RpZmY6UGhvdG9tZXRyaWNJbnRlcnByZXRhdGlvbj4KICAgICAgICAgPGV4aWY6UGl4ZWxZRGltZW5zaW9uPjkxMDwvZXhpZjpQaXhlbFlEaW1lbnNpb24+CiAgICAgICAgIDxleGlmOlBpeGVsWERpbWVuc2lvbj45MTA8L2V4aWY6UGl4ZWxYRGltZW5zaW9uPgogICAgICA8L3JkZjpEZXNjcmlwdGlvbj4KICAgPC9yZGY6UkRGPgo8L3g6eG1wbWV0YT4KYUWh7AAAAA1JREFUCB1jiJlX8B8ABS4CaoPQPXgAAAAASUVORK5CYII=' /></div>"
