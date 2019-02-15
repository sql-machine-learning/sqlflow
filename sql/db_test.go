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

func openSQLite3() (*sql.DB, *mysql.Config) {
	n := fmt.Sprintf("%d%d", time.Now().Unix(), os.Getpid())
	db, e := sql.Open("sqlite3", n)
	if e != nil {
		log.Fatalf("TestMain cannot connect to SQLite3: %q.", e)
	}
	return db, nil
}

func openMySQL() (*sql.DB, *mysql.Config) {
	cfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		log.Fatalf("TestMain cannot connect to MySQL: %q.\n"+
			"Please run MySQL server as in example/datasets/README.md.", e)
	}
	return db, cfg
}

func TestMain(m *testing.M) {
	dbms := flag.String("testdb", "mysql", "Choose the DBMS used for unit testing: sqlite3 or mysql")
	flag.Parse()

	switch *dbms {
	case "sqlite3":
		testDB, testCfg = openSQLite3()
	case "mysql":
		testDB, testCfg = openMySQL()
	default:
		log.Fatalf("Unrecognized commnad option value testdb=%s", *dbms)
	}
	defer testDB.Close()

	os.Exit(m.Run())
}
