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
	_ "image/png"

	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/step"

	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
	"sqlflow.org/sqlflow/pkg/tablewriter"
)

const tablePageSize = 1000

func isSpace(c byte) bool {
	return len(bytes.TrimSpace([]byte{c})) == 0
}

// addLineToStmt scans lines into statements, the last four parameters are both input/output.
// A user must initialize `inQuotedString` and `isSingleQuoted` to false and `statements` to []
// at the first call
func addLineToStmt(line string, inQuotedString, isSingleQuoted *bool, statements *[]string) bool {
	if len(*statements) == 0 { // First line of the statements
		*statements = append(*statements, "")
		line = strings.TrimLeft(line, "\t ")
	} else {
		(*statements)[len(*statements)-1] += "\n"
	}
	var isEscape bool // Escaping in quoted string cannot cross lines
	var start, i int

	// note(yancey1989): if the input sql program contain `\n`, bufio.Text() would deal with
	// it as a text string with two character instead of one character "\n".
	// readStmt(bufio.Scanner) should deal with that by replacing `\n` with '\n'
	// TODO(yancey1989): finding a normative way to deal with that.
	replacer := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r")
	line = replacer.Replace(line)

	for i = 0; i < len(line); i++ {
		if isEscape {
			isEscape = false
			continue
		}
		switch line[i] {
		case '\\':
			if *inQuotedString {
				isEscape = true
			}
		case '"', '\'':
			if *inQuotedString {
				if *isSingleQuoted == (line[i] == '\'') {
					*inQuotedString = false // We found the end of a quoted string
				}
			} else { // The start of a quoted string
				*inQuotedString = true
				*isSingleQuoted = (line[i] == '\'')
			}
		case ';':
			if !*inQuotedString { // We found a statement
				if i-start != 1 { // Ignore empty statement that has only a ';'
					(*statements)[len(*statements)-1] += line[start : i+1]
				}
				for i+1 < len(line) && isSpace(line[i+1]) {
					i++ // Ignore leading whitespaces of the next statement
				}
				start = i + 1
				if start == len(line) {
					return true // All done, the last character in the line is the end of a statement
				}
				*statements = append(*statements, "") // Prepare for searching the next statement

			}
		case '-':
			if !*inQuotedString {
				if i+1 < len(line) && line[i+1] == '-' {
					if i+2 == len(line) || isSpace(line[i+2]) { // We found a line comment
						// Note: `--` comment doesn't interfere with quoted-string and `;`
						(*statements)[len(*statements)-1] += strings.TrimSpace(line[start:i])
						if len(*statements) == 1 && (*statements)[0] == "" {
							*statements = []string{}
							return true // The whole line is an empty statement that has only a `-- comment`,
						}
						return false
					}
				}
			}
		}
	}
	(*statements)[len(*statements)-1] += line[start:]
	return false
}

// readStmt reads a SQL statement from the scanner.  A statement could have
// multiple lines and ends at a semicolon at the end of the last line.
func readStmt(scn *bufio.Scanner) ([]string, error) {
	stmt := []string{}
	var inQuotedString, isSingleQuoted bool
	for scn.Scan() {
		if addLineToStmt(scn.Text(), &inQuotedString, &isSingleQuoted, &stmt) {
			return stmt, nil
		}
	}
	// If the the file doesn't ends with ';', we consider the remaining content as a statement
	if scn.Err() == nil {
		return stmt, io.EOF
	}
	return stmt, scn.Err()
}

var it2Check = false

func flagPassed(name ...string) bool {
	found := false
	for _, n := range name {
		flag.Visit(func(f *flag.Flag) {
			if f.Name == n {
				found = true
			}
		})
	}
	return found
}

func runStmt(stmt string, isTerminal bool, modelDir string, ds string) error {
	if !isTerminal {
		fmt.Println("sqlflow>", stmt)
	}
	var table tablewriter.TableWriter
	var err error
	// TODO(yancey1989): remove protobuf tablewriter if using step binary in workflow
	if isWorkflowStep() {
		table, err = tablewriter.Create("protobuf", tablePageSize, os.Stdout)
	} else {
		table, err = tablewriter.Create("ascii", tablePageSize, os.Stdout)
	}
	if err != nil {
		return err
	}
	sess := sql.MakeSessionFromEnv()
	sess.DbConnStr = getDataSource(ds, currentDB)
	parts := strings.Fields(strings.ReplaceAll(stmt, ";", ""))
	if len(parts) == 2 && strings.ToUpper(parts[0]) == "USE" {
		return switchDatabase(parts[1], sess)
	}
	return step.RunSQLProgramAndPrintResult(stmt, modelDir, sess, table, isTerminal, it2Check)
}

func assertConnectable(ds string) {
	db, err := database.OpenAndConnectDB(ds)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
}

func repl(scanner *bufio.Scanner, modelDir string, ds string) {
	for {
		statements, err := readStmt(scanner)
		if err == io.EOF && len(statements) == 0 {
			return
		}
		// The collaborative parsing algorithm requires that each statement ends
		// with a semi-colon, as the definition of `end_of_stmt`
		// in /pkg/parser/extended_syntax_parser.y#L176 .
		n := len(statements)
		if n > 0 && !strings.HasSuffix(strings.TrimSpace(statements[n-1]), ";") {
			statements[len(statements)-1] += ";"
		}
		for _, stmt := range statements {
			if err := runStmt(stmt, false, modelDir, ds); err != nil {
				log.Fatalf("run SQL statement failed: %v", err)
			}
		}
	}
}

func switchDatabase(db string, session *pb.Session) error {
	stream := sql.RunSQLProgram("USE "+db, "", session)
	r := <-stream.ReadAll()
	switch r.(type) {
	case string:
		session.DbConnStr = getDataSource(session.DbConnStr, db)
		fmt.Println("Database changed to", db)
		currentDB = db
	case error:
		fmt.Println(r)
	}
	return nil
}

// getDataSource generates a data source string that is using database `db` from the original dataSource
func getDataSource(dataSource, db string) string {
	driver, other, e := database.ParseURL(dataSource)
	if e != nil {
		log.Fatalf("unrecognized data source '%s'", dataSource)
	}
	pieces := strings.Split(other, "?")
	switch driver {
	case "maxcompute", "alisa":
		var v url.Values = url.Values{}
		if len(pieces) == 2 {
			v, e = url.ParseQuery(pieces[1])
			if e != nil {
				log.Fatalf("unrecognized data source '%s'", dataSource)
			}
		}
		v["curr_project"] = []string{db}
		return fmt.Sprintf("%s://%s?%s", driver, pieces[0], v.Encode())
	case "mysql":
		fallthrough
	case "hive":
		pieces[0] = strings.Split(pieces[0], "/")[0] + "/" + db
		return fmt.Sprintf("%s://%s", driver, strings.Join(pieces, "?"))
	}
	log.Fatalf("unknown database '%s' in data source'%s'", driver, dataSource)
	return ""
}

var currentDB string

func main() {
	step.InitEnvFromFile(filepath.Join(os.Getenv("HOME"), step.DotEnvFilename))
	ds := flag.String("datasource", "", "database connect string")
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	cliStmt := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(cliStmt, "e", "", "execute SQLFlow from command line, short for --execute")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	flag.StringVar(sqlFileName, "f", "", "execute SQLFlow from file, short for --file")
	noAutoCompletion := flag.Bool("A", false, "No auto completion for sqlflow models. This gives a quicker start.")
	flag.Parse()
	if *ds == "" {
		*ds = os.Getenv("SQLFLOW_DATASOURCE")
	}
	assertConnectable(*ds) // Fast fail if we can't connect to the datasource
	var err error
	currentDB, err = database.GetDatabaseName(*ds)
	if err != nil {
		log.Fatalf("error SQLFLOW_DATASOURCE: %v", err)
	}

	if *modelDir != "" {
		if _, derr := os.Stat(*modelDir); derr != nil {
			os.Mkdir(*modelDir, os.ModePerm)
		}
	}

	isTerminal := !flagPassed("execute", "e", "file", "f") && terminal.IsTerminal(syscall.Stdin)
	sqlFile := os.Stdin

	if flagPassed("file", "f") {
		sqlFile, err = os.Open(*sqlFileName)
		if err != nil {
			log.Fatal(err)
		}
		defer sqlFile.Close()
	}
	var reader io.Reader = sqlFile
	// Override stdin and file when the `-e|-execute' options are present.
	if flagPassed("execute", "e") {
		reader = strings.NewReader(strings.TrimSpace(*cliStmt))
	}
	scanner := bufio.NewScanner(reader)
	if isTerminal {
		if !it2Check {
			fmt.Println("The terminal doesn't support sixel, explanation statements will show ASCII figures.")
		}
		if !*noAutoCompletion {
			attribute.ExtractDocStringsOnce()
		}
		runPrompt(func(stmt string) { runStmt(stmt, true, *modelDir, *ds) })
	} else {
		repl(scanner, *modelDir, *ds)
	}
}

func isWorkflowStep() bool {
	// note(yancey1989): the specified env would be set if repl running in Kubernetes Pod
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	return false
}

func init() {
	// `it2check` and `go-prompt` both set terminal to raw mode, we has to call `it2check` only once
	cmd := exec.Command("it2check")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if cmd.Run() == nil {
		it2Check = true
	}
}
