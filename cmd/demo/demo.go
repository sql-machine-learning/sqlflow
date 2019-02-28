package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	sf "gitlab.alipay-inc.com/Arc/sqlflow/sql"
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
	user := flag.String("db_user", "", "database user name")
	pswd := flag.String("db_password", "", "database user password")
	addr := flag.String("db_address", "", "database address, such as: localhost:3306")
	flag.Parse()

	cfg := &mysql.Config{
		User:   *user,
		Passwd: *pswd,
		Addr:   *addr,
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

		stream := sf.Run(slct, db)
		for rsp := range stream.ReadAll() {
			display(rsp)
		}
	}
}
