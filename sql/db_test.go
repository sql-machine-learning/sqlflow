package sql

import (
	"bufio"
	"database/sql"
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
	fmt.Println(os.Getenv("SQLFLOW_TEST_DB"))
	switch os.Getenv("SQLFLOW_TEST_DB") {
	case "sqlite3":
		testDB, testCfg = openSQLite3()
		fmt.Println("opened sqlite3")
		defer testDB.Close()
	case "mysql":
		testDB, testCfg = openMySQL()
		defer testDB.Close()
	default:
		log.Fatalf("Unrecognized environment variable value SQLFLOW_TEST_DB==%s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	fmt.Println("opened db")
	popularize(testDB, "testdata/iris.sql")
	popularize(testDB, "testdata/churn.sql")
	fmt.Println("popularized")
	os.Exit(m.Run())
}

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
