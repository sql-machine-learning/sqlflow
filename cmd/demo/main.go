package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wangkuiyi/sqlflow/sql"
)

func run(slct string) (string, error) {
	testCfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}

	if strings.Contains(slct, "TRAIN") || strings.Contains(slct, "PREDICT") {
		if err := sql.Run(slct, testCfg); err != nil {
			return "", err
		}
		return "Job success", nil
	}

	return runStandardSQL(slct, testCfg)
}

func runStandardSQL(slct string, cfg *mysql.Config) (string, error) {
	cmd := exec.Command("docker", "exec", "-t",
		// set password as envirnment variable to surpress warnings
		// https://stackoverflow.com/a/24188878/6794675
		"-e", fmt.Sprintf("MYSQL_PWD=%s", cfg.Passwd),
		"sqlflowtest",
		"mysql", fmt.Sprintf("-u%s", cfg.User),
		"-e", fmt.Sprintf("%s", slct))
	o, e := cmd.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("runStandardSQL failed %v: \n%s", e, o)
	}
	return string(o), nil
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
