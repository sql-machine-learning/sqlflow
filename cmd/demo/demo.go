package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	tablewriter "github.com/olekukonko/tablewriter"
	sf "gitlab.alipay-inc.com/Arc/sqlflow/sql"
)

const TABLE_PAGE_SIZE = 1000

// readStmt reads a SQL statement from the scanner.  A statement could
// have multiple lines and ends at a semicolon at theend of the last
// line.
func readStmt(scn *bufio.Scanner) string {
	stmt := ""
	for scn.Scan() {
		stmt += scn.Text() + "\n"
		if strings.HasSuffix(strings.TrimSpace(scn.Text()), ";") {
			break
		}
	}
	return stmt
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

func render(rsp interface{}, isTable *bool, table *tablewriter.Table) {
	switch s := rsp.(type) {
	case map[string]interface{}: // table header
		*isTable = true
		cols, e := header(s)
		if e == nil {
			table.SetHeader(cols)
		}
	case []interface{}: // row
		row := make([]string, len(s))
		for i, v := range s {
			row[i] = fmt.Sprint(v)
		}
		table.Append(row)
	case error:
		fmt.Printf("ERROR: %v\n", s)
	default:
		fmt.Println(s)
	}
}

func main() {
	user := flag.String("db_user", "", "database user name")
	pswd := flag.String("db_password", "", "database user password")
	addr := flag.String("db_address", "", "database address, such as: localhost:3306")
	flag.Parse()

	cfg := &mysql.Config{
		User:   *user,
		Passwd: *pswd,
		Net:    "tcp",
		Addr:   *addr,
		// Allow the usage of the mysql native password method
		AllowNativePasswords: true,
	}
	db, err := sf.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	scn := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("sqlflow> ")
		slct := readStmt(scn)
		fmt.Println("-----------------------------")

		isTable := false
		tableReadered := false
		table := tablewriter.NewWriter(os.Stdout)

		stream := sf.Run(slct, db)
		for rsp := range stream.ReadAll() {
			render(rsp, &isTable, table)

			// pagination. avoid exceed memory
			if isTable && table.NumLines() == TABLE_PAGE_SIZE {
				table.Render()
				tableReadered = true
				table.ClearRows()
			}
		}
		if isTable && (table.NumLines() > 0 || !tableReadered) {
			table.Render()
		}
	}
}
