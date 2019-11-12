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
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/crypto/ssh/terminal"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
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

func render(rsp interface{}, table *tablewriter.Table) bool {
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
		fmt.Printf("ERROR: %v\n", s)
	case sql.EndOfExecution:
		return isTable
	default:
		fmt.Println(s)
	}
	return isTable
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

func runStmt(stmt string, isTerminal bool, modelDir string, db *sql.DB, ds string) {
	if !isTerminal {
		fmt.Println("sqlflow>", stmt)
	}
	isTable, tableRendered := false, false
	table := tablewriter.NewWriter(os.Stdout)

	cwd, err := ioutil.TempDir("/tmp", "sqlflow")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(cwd)

	stream := sql.RunSQLProgram([]string{stmt}, db, modelDir, &pb.Session{})
	for rsp := range stream.ReadAll() {
		isTable = render(rsp, table)

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
}

func repl(scanner *bufio.Scanner, isTerminal bool, modelDir string, db *sql.DB, ds string) {
	for {
		if isTerminal {
			fmt.Print("sqlflow> ")
		}
		stmt, err := readStmt(scanner)
		fmt.Println()
		if err == io.EOF && stmt == "" {
			return
		}
		runStmt(stmt, isTerminal, modelDir, db, ds)
	}

}

func main() {
	ds := flag.String("datasource", "", "database connect string")
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	cliStmt := flag.String("execute", "", "execute SQLFlow from command line.  e.g. --execute 'select * from table1'")
	flag.StringVar(cliStmt, "e", "", "execute SQLFlow from command line, short for --execute")
	sqlFileName := flag.String("file", "", "execute SQLFlow from file.  e.g. --file '~/iris_dnn.sql'")
	flag.StringVar(sqlFileName, "f", "", "execute SQLFlow from file, short for --file")
	flag.Parse()
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
	repl(scanner, isTerminal, *modelDir, db, *ds)
}
