package main

import (
	"bufio"
	sql "database/sql"
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
		stmt += strings.TrimSpace(scn.Text())
		if strings.HasSuffix(stmt, ";") {
			break
		}
	}
	return stmt
}

func main() {
	testCfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		log.Fatalf("This demo cannot connect to a MySQl server listening on port 3306")
	}
	defer db.Close()

	scn := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("sqlflow> ")
		slct := readStmt(scn)
		fmt.Println("-----------------------------")

		stream := sqlflow.Run(slct, db)
		for rsp := range stream {
			fmt.Println(rsp)
		}
	}
}
