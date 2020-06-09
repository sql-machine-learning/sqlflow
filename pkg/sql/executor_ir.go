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

package sql

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/log"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/pipe"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

// EndOfExecution will push to the pipe when one SQL statement execution is finished.
type EndOfExecution struct {
	StartTime int64
	EndTime   int64
	Statement string
}

// RunSQLProgram run a SQL program.
//
// TODO(wangkuiyi): Make RunSQLProgram return an error in addition to
// *pipe.Reader, and remove the calls to log.Printf.
func RunSQLProgram(sqlProgram string, modelDir string, session *pb.Session) *pipe.Reader {
	rd, wr := pipe.Pipe()
	go func() {
		var err error
		defer wr.Close()
		err = runSQLProgram(wr, sqlProgram, modelDir, session)
		if err != nil {
			if e := wr.Write(fmt.Errorf("runSQLProgram error: %v", err)); e != nil {
				log.GetDefaultLogger().Errorf("runSQLProgram error(piping): %v", e)
			}
		}
	}()
	return rd
}

// ResolveSQLProgram accepts parse result from parser and returns a `pb.Program`
func ResolveSQLProgram(sqlStmts []*parser.SQLFlowStmt, session *pb.Session, logger *log.Logger) (*pb.Program, error) {
	ret := pb.Program{Datasource: session.DbConnStr}
	for _, sql := range sqlStmts {
		stmt, err := ir.GenerateStatement(sql)
		logger.Info("resolveSQL:", stmt.Type)
		if err != nil {
			return nil, err
		}
		logger.Infof("Original SQL is:%s", stmt.OriginalSql)
		ret.Statements = append(ret.Statements, stmt)
	}
	return &ret, nil
}

func runSQLProgram(wr *pipe.Writer, sqlProgram string, modelDir string, session *pb.Session) error {
	driver, _, err := database.ParseURL(session.DbConnStr)
	if err != nil {
		return err
	}
	stmts, err := parser.Parse(driver, sqlProgram)
	if err != nil {
		return err
	}
	// NOTE(tony): We generate IR and execute its translated program one-by-one since IR generation may depend on the execution
	// of the previous statement. For example, consider a SQL program
	//
	//   create table some_table as (select ...);
	//   select * from some_table to train ...
	//
	// The IR generation on the second statement would fail since it requires inspection the schema of some_table,
	// which depends on the execution of create table some_table as (select ...);.
	sqls := RewriteStatementsWithHints(stmts, driver)
	for _, sql := range sqls {
		if err := runSingleSQLFlowStatement(wr, sql, modelDir, session); err != nil {
			return err
		}
	}
	return nil
}

func runSingleSQLFlowStatement(wr *pipe.Writer, sql *parser.SQLFlowStmt, modelDir string, session *pb.Session) (e error) {
	defer func(startTime int64) {
		// NOTE(tony): EndOfExecution indicates a successful run,
		// so we only writes it when e != nil
		if e != nil {
			wr.Write(EndOfExecution{
				StartTime: startTime,
				EndTime:   time.Now().UnixNano(),
				Statement: sql.Original,
			})
		}
	}(time.Now().UnixNano())

	// use system default tmp dir
	cwd, err := ioutil.TempDir("", "sqlflow_models")
	if err != nil {
		return err
	}
	defer func(cwd string) {
		if err := os.RemoveAll(cwd); err != nil {
			e = fmt.Errorf("encounter %v when dealwith error: %s", e, err)
		}
	}(cwd)
	cmd := exec.Command("python", "-u", "-m", "sqlflow.execute", "step")
	cmd.Dir = cwd
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	wStdout := bufio.NewWriter(&stdout)
	wStderr := bufio.NewWriter(&stderr)
	stmt, err := ir.GenerateStatement(sql)
	if err != nil {
		return err
	}
	program := pb.Program{Statements: []*pb.Statement{stmt}, Datasource: session.DbConnStr}
	t := proto.TextMarshaler{}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewBufferString(t.Text(&program)), wStdout, wStderr
	if e := cmd.Run(); e != nil {
		// return the diagnostic message
		sub := rePyDiagnosis.FindStringSubmatch(stderr.String())
		if len(sub) == 2 {
			return fmt.Errorf("%s", sub[1])
		}
		// if no diagnostic message, return the full stack trace
		return fmt.Errorf("failed: %v\n%sGenerated Code:%[2]s\n%s\n%[2]sOutput%[2]s\n%[4]v",
			e, "==========", t.Text(&program), stdout.String()+stderr.String())
	}
	if stmt.Type == pb.Statement_EXPLAIN {
		return readExplainResult(cwd, wr)
	}
	scn := bufio.NewScanner(bufio.NewReader(&stdout))
	isTable := false
	for scn.Scan() {
		if !isTable {
			var head []string
			e = json.Unmarshal([]byte(scn.Text()), &head)
			if e != nil {
				wr.Write(scn.Text())
				continue
			}
			wr.Write(map[string]interface{}{"columnNames": head})
			isTable = true
		} else {
			var row []interface{}
			e = json.Unmarshal([]byte(scn.Text()), &row)
			if e != nil {
				return fmt.Errorf("malformed output format")
			}
			wr.Write(row)
		}
	}
	return scn.Err()
}

// RewriteStatementsWithHints combines the hints into the standard SQL(s)
//
// FIXME(weiguoz): I'm not happy with such an implementation.
// I mean it is not clean that sqlflow handles such database relative details.
func RewriteStatementsWithHints(stmts []*parser.SQLFlowStmt, dialect string) []*parser.SQLFlowStmt {
	hints, sqls := splitHints(stmts, dialect)
	if len(hints) > 0 {
		for _, sql := range sqls {
			if !sql.IsExtendedSyntax() {
				sql.Original = hints + sql.Original
			}
		}
	}
	return sqls
}

func splitHints(stmts []*parser.SQLFlowStmt, dialect string) (string, []*parser.SQLFlowStmt) {
	hints, sqls := "", []*parser.SQLFlowStmt{}
	for _, stmt := range stmts {
		if isHint(stmt, dialect) {
			hints += stmt.Original + "\n" // alisa's requirements
		} else {
			sqls = append(sqls, stmt)
		}
	}
	return hints, sqls
}

func isHint(stmt *parser.SQLFlowStmt, dialect string) bool {
	if !stmt.IsExtendedSyntax() {
		if dialect == "alisa" {
			return isAlisaHint(stmt.Original)
		}
		// TODO(weiguoz) handle if submitter is "maxcompute" or "hive"
	}
	return false
}

func isAlisaHint(sql string) bool {
	for {
		sql = strings.TrimSpace(sql)
		// TODO(weiguoz): Let's remove the following code if we clean the comments before
		if strings.HasPrefix(sql, "--") {
			eol := strings.IndexAny(sql, "\n\r")
			if eol != -1 {
				sql = sql[eol+1:]
			} else {
				break
			}
		} else {
			break
		}
	}
	return strings.HasPrefix(strings.ToLower(sql), "set ")
}
