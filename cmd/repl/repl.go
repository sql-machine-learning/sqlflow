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
	"image"
	_ "image/png"

	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
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
)

const tablePageSize = 1000

// readStmt reads a SQL statement from the scanner.  A statement could have
// multiple lines and ends at a semicolon at the end of the last line.
func readStmt(scn *bufio.Scanner) (string, error) {
	stmt := ""
	for scn.Scan() {
		stmt += scn.Text()
		// FIXME(tonyyang-svail): It is hacky and buggy to assume that
		// SQL statements are separated by substrings ";\n".  We need
		// to call the SQLFlow parser to retrieve statements and run
		// them one-by-one in a REPL.
		if strings.HasSuffix(strings.TrimSpace(scn.Text()), ";") {
			return strings.TrimSpace(stmt), nil
		}
		stmt += "\n"
	}
	if scn.Err() == nil {
		return stmt, io.EOF
	}
	return "", scn.Err()
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
	if ds != "" {
		sess.DbConnStr = ds
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
		stmt, err := readStmt(scanner)
		fmt.Println()
		if err == io.EOF && stmt == "" {
			return
		}
		if err := runStmt(stmt, false, modelDir, ds); err != nil {
			log.Fatalf("run SQL statement failed: %v", err)
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
		HdfsUser:         os.Getenv("JUPYTER_HADOOP_USER"),
		HdfsPass:         os.Getenv("JUPYTER_HADOOP_PASS"),
	}
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func main() {
	ds := flag.String("datasource", "", "database connect string")
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	cliStmt := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(cliStmt, "e", "", "execute SQLFlow from command line, short for --execute")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	flag.StringVar(sqlFileName, "f", "", "execute SQLFlow from file, short for --file")
	flag.Parse()

	assertConnectable(*ds) // Fast fail if we can't connect to the datasource

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
		if !commandExists("it2check") {
			fmt.Println("Warning: defaults to non-sixel mode")
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
