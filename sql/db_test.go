package sql

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func TestMain(m *testing.M) {
	dbms := os.Getenv("SQLFLOW_TEST_DB")
	if dbms == "" {
		dbms = "mysql"
	}

	var e error
	switch dbms {
	case "sqlite3":
		testDB, testCfg, e = openSQLite3()
		defer testDB.Close()
	case "mysql":
		testDB, testCfg, e = openMySQL()
		defer testDB.Close()
	default:
		e = fmt.Errorf("Unrecognized environment variable SQLFLOW_TEST_DB %s\n", dbms)
	}
	assertNoErr(e)

	assertNoErr(popularize(testDB, "testdata/iris.sql"))
	assertNoErr(popularize(testDB, "testdata/churn.sql"))

	os.Exit(m.Run())
}

// assertNoError prints the error if there is any in TestMain, which
// log doesn't work.
func assertNoErr(e error) {
	if e != nil {
		fmt.Println(e)
		os.Exit(-1)
	}
}

func openSQLite3() (*sql.DB, *mysql.Config, error) {
	n := fmt.Sprintf("%d%d", time.Now().Unix(), os.Getpid())
	db, e := sql.Open("sqlite3", n)
	return db, nil, e
}

func openMySQL() (*sql.DB, *mysql.Config, error) {
	cfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	return db, cfg, e
}

// popularize reads SQL statements from the file named sqlfile in the
// ./testdata directory, and runs each SQL statement with db.
func popularize(db *sql.DB, sqlfile string) error {
	f, e := os.Open(sqlfile)
	if e != nil {
		return e
	}
	defer f.Close()

	onSemicolon := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		for i := 0; i < len(data); i++ {
			if data[i] == ';' {
				return i + 1, data[:i], nil
			}
		}
		return 0, nil, nil
	}

	scanner := bufio.NewScanner(f)
	scanner.Split(onSemicolon)

	for scanner.Scan() {
		_, e := db.Exec(scanner.Text())
		if e != nil {
			return e
		}
	}
	return scanner.Err()
}
