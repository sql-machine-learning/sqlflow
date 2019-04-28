package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"sqlflow.org/sqlflow/sql"
)

const tablePageSize = 1000

// readStmt reads a SQL statement from the scanner.  A statement could have
// multiple lines and ends at a semicolon at theend of the last line.
func readStmt() string {
	stmt := ""
	scn := bufio.NewScanner(os.Stdin)
	for scn.Scan() {
		stmt += scn.Text()
		if strings.HasSuffix(strings.TrimSpace(scn.Text()), ";") {
			break
		}
		stmt += "\n"
	}
	if scn.Err() != nil {
		return ""
	}
	return strings.TrimSpace(stmt)
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
	default:
		fmt.Println(s)
	}
	return isTable
}

func main() {
	ds := flag.String("datasource", "", "database connect string")
	flag.Parse()
	db, err := sql.Open(*ds)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	for {
		fmt.Print("sqlflow> ")
		slct := readStmt()
		fmt.Println("")

		isTable, tableRendered := false, false
		table := tablewriter.NewWriter(os.Stdout)

		stream := sql.Run(slct, db)
		for rsp := range stream.ReadAll() {
			isTable = render(rsp, table)

			// pagination. avoid exceed memory
			if isTable && table.NumLines() == tablePageSize {
				table.Render()
				tableRendered = true
				table.ClearRows()
			}
		}
		if isTable && (table.NumLines() > 0 || !tableRendered) {
			table.Render()
		}
		fmt.Println("")
	}
}
