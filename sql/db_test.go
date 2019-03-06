package sql

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testDB *DB
)

func TestMain(m *testing.M) {
	dbms := os.Getenv("SQLFLOW_TEST_DB")
	if dbms == "" {
		dbms = "mysql"
	}

	var e error
	switch dbms {
	case "sqlite3":
		testDB, e = Open("sqlite3", ":memory:")
		assertNoErr(e)
		// attach an In-Memory Database in SQLite
		for _, name := range []string{"iris", "churn"} {
			_, e = testDB.Exec(fmt.Sprintf("ATTACH DATABASE ':memory:' AS %s;", name))
			assertNoErr(e)
		}
		defer testDB.Close()
	case "mysql":
		cfg := &mysql.Config{
			User:                 "root",
			Passwd:               "root",
			Net:                  "tcp",
			Addr:                 "localhost:3306",
			AllowNativePasswords: true,
		}
		testDB, e = Open("mysql", cfg.FormatDSN())
		assertNoErr(e)
		_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS iris;")
		assertNoErr(e)
		_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS churn;")
		assertNoErr(e)
		defer testDB.Close()
	default:
		e = fmt.Errorf("unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
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

// popularize reads SQL statements from the file named sqlfile in the
// ./testdata directory, and runs each SQL statement with db.
func popularize(db *DB, sqlfile string) error {
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
