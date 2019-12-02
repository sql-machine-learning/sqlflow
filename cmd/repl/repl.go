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
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/crypto/ssh/terminal"
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

func render(rsp interface{}, table *tablewriter.Table) (bool, error) {
	isTable := false
	switch s := rsp.(type) {
	case map[string]interface{}: // table header
		cols, e := header(s)
		if e == nil {
			table.SetHeader(cols)
		}
		isTable = true
	case []interface{}: // row
		row := make([]string, len(s))
		for i, v := range s {
			row[i] = fmt.Sprint(v)
		}
		table.Append(row)
		isTable = true
	case error:
		return false, s
	case sql.EndOfExecution:
		return isTable, nil
	default:
		fmt.Println(s)
	}
	return isTable, nil
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

func runStmt(stmt string, isTerminal bool, modelDir string, db *sql.DB, ds string) error {
	if !isTerminal {
		fmt.Println("sqlflow>", stmt)
	}
	isTable, tableRendered := false, false
	var err error
	table := tablewriter.NewWriter(os.Stdout)
	sess := makeSessionFromEnv()

	stream := sql.RunSQLProgram(stmt, db, modelDir, sess)
	for rsp := range stream.ReadAll() {
		isTable, err = render(rsp, table)
		if err != nil {
			return err
		}

		// pagination. avoid exceed memory
		if isTable && table.NumLines() == tablePageSize {
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

func repl(scanner *bufio.Scanner, modelDir string, db *sql.DB, ds string) {
	for {
		stmt, err := readStmt(scanner)
		fmt.Println()
		if err == io.EOF && stmt == "" {
			return
		}
		if err := runStmt(stmt, false, modelDir, db, ds); err != nil {
			log.Fatalf("run SQL statment failed: %v", err)
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

func parseSQLFromStdin(stdin io.Reader) (string, error) {
	scanedInput := []string{}
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		scanedInput = append(scanedInput, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	sqlflowDatasrouce := os.Getenv("SQLFLOW_DATASOURCE")
	if sqlflowDatasrouce == "" {
		return "", fmt.Errorf("no SQLFLOW_DATASOURCE env provided")
	}
	sess := makeSessionFromEnv()
	pbIRStr, err := sql.ParseSQLStatement(strings.Join(scanedInput, ""), sess)
	if err != nil {
		return "", err
	}
	return pbIRStr, nil
}

func main() {
	ds := flag.String("datasource", "", "database connect string")
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	cliStmt := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(cliStmt, "e", "", "execute SQLFlow from command line, short for --execute")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	isParseOnly := flag.Bool("parse", false, "execute parsing only and output the parsed IR in pbtxt format")
	flag.StringVar(sqlFileName, "f", "", "execute SQLFlow from file, short for --file")
	flag.Parse()
	// Read SQL from stdin and output IR in pbtxt format
	// Assume the input is a single SQL statement
	if *isParseOnly {
		out, err := parseSQLFromStdin(os.Stdin)
		if err != nil {
			log.Fatalf("error parse SQL from stdin: %v", err)
		}
		fmt.Printf("%s", out)
		// exit when parse is finished
		os.Exit(0)
	}

	db, err := sql.NewDB(*ds)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

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
		reader = strings.NewReader(*cliStmt)
	}
	scanner := bufio.NewScanner(reader)
	if isTerminal {
		runPrompt(func(stmt string) { runStmt(stmt, true, *modelDir, db, *ds) })
	} else {
		repl(scanner, *modelDir, db, *ds)
	}
}
