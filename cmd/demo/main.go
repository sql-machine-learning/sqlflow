package main

import (
	"bufio"
	sql "database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	sqlflow "github.com/wangkuiyi/sqlflow/sql"
)

func readStmt(scn *(bufio.Scanner)) string {
	var lines []string
	for scn.Scan() {
		line := scn.Text()
		if strings.Contains(line, ";") {
			lines = append(lines, line)
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func main() {
	testCfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		return
	}
	defer db.Close()

	scn := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("sqlflow> ")
		slct := readStmt(scn)
		fmt.Println("-----------------------------")

		rsp, e := sqlflow.Run(slct, db, testCfg)
		if e != nil {
			fmt.Println(e.Error())
		} else {
			for r := range rsp {
				fmt.Println(r)
			}
		}
	}
}
