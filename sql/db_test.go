package sql

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

var (
	testDB *Database
)

func TestMain(m *testing.M) {
	dbms := os.Getenv("SQLFLOW_TEST_DB")
	if dbms == "" {
		dbms = "mysql"
	}

	var e error
	switch dbms {
	case "sqlite3":
		testDB = &Database{
			DataSource: fmt.Sprintf("%d%d", time.Now().Unix(), os.Getpid()),
		}
		e = testDB.Open()
		defer testDB.Close()
	case "mysql":
		testDB = &Database{
			User:       "root",
			Password:   "root",
			Addr:       "localhost:3306",
			DriverName: "mysql",
		}
		e = testDB.Open()
		defer testDB.Close()
	default:
		e = fmt.Errorf("Unrecognized environment variable SQLFLOW_TEST_DB %s\n", dbms)
	}
	assertNoErr(e)

	assertNoErr(popularize(testDB.Conn, "testdata/iris.sql"))
	assertNoErr(popularize(testDB.Conn, "testdata/churn.sql"))

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
