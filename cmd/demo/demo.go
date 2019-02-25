package main

import (
	"bufio"
	sql "database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	sqlflow "gitlab.alipay-inc.com/Arc/sqlflow/sql"
)

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

func displayHead(head map[string]interface{}) {
	cn, ok := head["columnNames"]
	if !ok {
		fmt.Print("ERROR: can't find field columnNames in head")
	}
	cols, ok := cn.([]string)
	if !ok {
		fmt.Print("ERROR: invalid header type")
	}
	for _, ele := range cols {
		fmt.Printf("%15s", ele)
	}
	fmt.Println()
}

func displayRow(row []interface{}) {
	for _, ele := range row {
		fmt.Printf("%15v", ele)
	}
	fmt.Println()
}

func display(rsp interface{}) {
	switch s := rsp.(type) {
	case map[string]interface{}:
		displayHead(s)
	case []interface{}:
		displayRow(s)
	case error:
		fmt.Printf("ERROR: %v\n", s)
	default:
		fmt.Println(s)
	}
}

func main() {
	addr := flag.String("addr", "localhost:3306", "MySQL server network adress")
	user := flag.String("user", "root", "Username of MySQL server")
	passwd := flag.String("passwd", "root", "Password of MySQL server")
	flag.Parse()

	cfg := &mysql.Config{
		User:   *user,
		Passwd: *passwd,
		Addr:   *addr}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		log.Fatalf("Cannot connect to a MySQL server %v", cfg)
	}
	defer db.Close()

	scn := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("sqlflow> ")
		slct := readStmt(scn)
		fmt.Println("-----------------------------")

		stream := sqlflow.Run(slct, db)
		for rsp := range stream.ReadAll() {
			display(rsp)
		}
	}
}
