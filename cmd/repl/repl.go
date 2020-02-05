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
	"image"
	_ "image/png"

	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/mattn/go-sixel"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/crypto/ssh/terminal"
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sql"
	"sqlflow.org/sqlflow/pkg/sql/codegen/attribute"
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

func header(head map[string]interface{}) ([]string, error) {
	cn, ok := head["columnNames"]
	if !ok {
		return nil, fmt.Errorf("can't find field columnNames in head")
	}
	cols, ok := cn.([]string)
	if !ok {
		return nil, fmt.Errorf("invalid header type")
	}
	return cols, nil
}

func isHTMLSnippet(s string) bool {
	// TODO(shendiaomo): more accurate checks later
	return strings.HasPrefix(s, "<div")
}

func printAsDataURL(s string) {
	fmt.Println("data:text/html,", s)
	fmt.Println()
	fmt.Println("To view the content, paste the above data url to a web browser.")
}

func getBase64EncodedImage(s string) ([]byte, error) {
	match := regexp.MustCompile(`base64,(.*)'`).FindStringSubmatch(s)
	if len(match) == 2 {
		return base64.StdEncoding.DecodeString(match[1])
	}
	return []byte{}, fmt.Errorf("no images in the HTML")
}

func imageCat(imageBytes []byte) error {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return err
	}
	err = sixel.NewEncoder(os.Stdout).Encode(img)
	if err != nil {
		return err
	}
	fmt.Println()
	return nil
}

var it2Check = false

func render(rsp interface{}, table *tablewriter.Table, isTerminal bool) bool {
	switch s := rsp.(type) {
	case map[string]interface{}: // table header
		cols, e := header(s)
		if e == nil {
			table.SetHeader(cols)
		}
		return true
	case []interface{}: // row
		row := make([]string, len(s))
		for i, v := range s {
			row[i] = fmt.Sprint(v)
		}
		table.Append(row)
		return true
	case error:
		if os.Getenv("SQLFLOW_log_dir") != "" { // To avoid printing duplicated error message to console
			log.New(os.Stderr, "", 0).Printf("ERROR: %v\n", s)
		}
		if !isTerminal {
			os.Exit(1)
		}
	case sql.EndOfExecution:
	case sql.Figures:
		if isHTMLSnippet(s.Image) {
			if !isTerminal {
				printAsDataURL(s.Image)
				break
			}
			if image, e := getBase64EncodedImage(s.Image); e != nil {
				printAsDataURL(s.Image)
			} else if !it2Check {
				printAsDataURL(s.Image)
				fmt.Println("Or use iTerm2 as your terminal to view images.")
				fmt.Println(s.Text)
			} else if e = imageCat(image); e != nil {
				log.New(os.Stderr, "", 0).Printf("ERROR: %v\n", e)
				printAsDataURL(s.Image)
				fmt.Println(s.Text)
			}
		} else {
			fmt.Println(s)
		}
	case string:
		fmt.Println(s)
	default:
		log.Fatalf("unrecognized response type: %v", s)
	}
	return false
}

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
	tableRendered := false
	table := tablewriter.NewWriter(os.Stdout)
	sess := makeSessionFromEnv()
	sess.DbConnStr = getDataSource(ds, currentDB)
	parts := strings.Fields(strings.ReplaceAll(stmt, ";", ""))
	if len(parts) == 2 && strings.ToUpper(parts[0]) == "USE" {
		return switchDatabase(parts[1], sess)
	}
	stream := sql.RunSQLProgram(stmt, modelDir, sess)
	for rsp := range stream.ReadAll() {
		// pagination. avoid exceed memory
		if render(rsp, table, isTerminal) && table.NumLines() == tablePageSize {
			table.Render()
			tableRendered = true
			table.ClearRows()
		}
	}
	if table.NumLines() > 0 || !tableRendered {
		table.Render()
	}
	return nil
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
		fmt.Println()
		if err == io.EOF && len(statements) == 0 {
			return
		}
		for _, stmt := range statements {
			if err := runStmt(stmt, false, modelDir, ds); err != nil {
				log.Fatalf("run SQL statement failed: %v", err)
			}
		}
	}
}

func makeSessionFromEnv() *pb.Session {
	return &pb.Session{
		Token:            os.Getenv("SQLFLOW_USER_TOKEN"),
		DbConnStr:        os.Getenv("SQLFLOW_DATASOURCE"),
		ExitOnSubmit:     strings.ToLower(os.Getenv("SQLFLOW_EXIT_ON_SUBMIT")) == "true",
		UserId:           os.Getenv("SQLFLOW_USER_ID"),
		HiveLocation:     os.Getenv("SQLFLOW_HIVE_LOCATION"),
		HdfsNamenodeAddr: os.Getenv("SQLFLOW_HDFS_NAMENODE_ADDR"),
		HdfsUser:         os.Getenv("SQLFLOW_HADOOP_USER"),
		HdfsPass:         os.Getenv("SQLFLOW_HADOOP_PASS"),
		Submitter:        os.Getenv("SQLFLOW_submitter")}
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

func getDatabaseName(datasource string) string {
	driver, other, e := database.ParseURL(datasource)
	if e != nil {
		log.Fatalf("unrecognized data source '%s'", datasource)
	}
	// The data source string of MySQL and Hive have similar patterns
	// with the database name as a pathname under root. For example:
	// mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0
	// hive://root:root@127.0.0.1:10000/iris?auth=NOSASL
	re := regexp.MustCompile(`[^/]*/(\w*).*`) // Extract the database name of MySQL and Hive
	switch driver {
	case "maxcompute":
		// The database name in data source string of MaxCompute is the argument to parameter
		// `curr_project`
		re = regexp.MustCompile(`[^/].*/api[?].*curr_project=(\w*).*`)
	case "mysql":
	case "hive":
	case "alisa": // TODO(yaney1989): using go drivers to parse the database
	default:
		log.Fatalf("unknown database '%s' in data source'%s'", driver, datasource)
	}
	if group := re.FindStringSubmatch(other); group != nil {
		return group[1]
	}
	return ""
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
		return fmt.Sprintf("maxcompute://%s?%s", pieces[0], v.Encode())
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
	ds := flag.String("datasource", "", "database connect string")
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	cliStmt := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(cliStmt, "e", "", "execute SQLFlow from command line, short for --execute")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	flag.StringVar(sqlFileName, "f", "", "execute SQLFlow from file, short for --file")
	noAutoCompletion := flag.Bool("A", false, "No auto completion for sqlflow models. This gives a quicker start.")
	flag.Parse()

	assertConnectable(*ds) // Fast fail if we can't connect to the datasource
	currentDB = getDatabaseName(*ds)

	if *modelDir != "" {
		if _, derr := os.Stat(*modelDir); derr != nil {
			os.Mkdir(*modelDir, os.ModePerm)
		}
	}

	isTerminal := !flagPassed("execute", "e", "file", "f") && terminal.IsTerminal(syscall.Stdin)
	sqlFile := os.Stdin
	var err error
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
		reader = strings.NewReader(*cliStmt)
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

func init() {
	// `it2check` and `go-prompt` both set terminal to raw mode, we has to call `it2check` only once
	cmd := exec.Command("it2check")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if cmd.Run() == nil {
		it2Check = true
	}
}
