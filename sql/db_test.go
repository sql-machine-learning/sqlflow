package sql

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func TestMain(m *testing.M) {
	dbms := flag.String("testdb", "sqlite3", "Choose the DBMS used for unit testing: sqlite3 or mysql")
	flag.Parse()

	switch *dbms {
	case "sqlite3":
		n := fmt.Sprintf("%d%d", time.Now().Unix(), os.Getpid())
		db, e := sql.Open("sqlite3", n)
		if e != nil {
			log.Fatalf("TestMain cannot connect to MySQL: %q.\n"+
				"Please run MySQL server as in example/churn/README.md.", e)
		}
		testDB = db
		testCfg = nil
	case "mysql":
		testCfg = &mysql.Config{
			User:   "root",
			Passwd: "root",
			Addr:   "localhost:3306",
		}
		db, e := sql.Open("mysql", testCfg.FormatDSN())
		if e != nil {
			log.Fatalf("TestMain cannot connect to MySQL: %q.\n"+
				"Please run MySQL server as in example/churn/README.md.", e)
		}
		testDB = db
	default:
		log.Fatalf("Unrecognized commnad option value testdb=%s", *dbms)
	}
	defer testDB.Close()

	os.Exit(m.Run())
}
