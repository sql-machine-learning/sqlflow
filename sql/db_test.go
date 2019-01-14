package sql

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func TestMain(m *testing.M) {
	testCfg = &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		fmt.Println("TestMain cannot connect to MySQL: %q.\n"+
			"Please run MySQL server as in example/churn/README.md.", e)
		os.Exit(-1)
	}
	testDB = db

	defer testDB.Close()
	os.Exit(m.Run())
}
