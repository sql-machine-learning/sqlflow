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

func run(slct string) (string, error) {
	testCfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		return "open mysql failed", e
	}
	defer db.Close()
	return sqlflow.Run(slct, db, testCfg)
}

func main() {
	scn := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("sqlflow> ")
		var lines []string
		for scn.Scan() {
			line := scn.Text()
			if strings.Contains(line, ";") {
				lines = append(lines, line)
				break
			}
			lines = append(lines, line)
		}
		fmt.Println("-----------------------------")
		slct := strings.Join(lines, "\n")

		s, e := run(slct)
		if e != nil {
			fmt.Println(e.Error())
		} else {
			fmt.Println(s)
		}
	}
}
