package sql

import (
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
	dbms := getEnv("SQLFLOW_TEST_DB", "mysql")

	var e error
	switch dbms {
	case "sqlite3":
		testDB, e = Open("sqlite3://:memory:")
		assertNoErr(e)
		// attach an In-Memory Database in SQLite
		for _, name := range []string{"iris", "churn"} {
			_, e = testDB.Exec(fmt.Sprintf("ATTACH DATABASE ':memory:' AS %s;", name))
			assertNoErr(e)
		}
		defer testDB.Close()
	case "mysql":
		cfg := &mysql.Config{
			User:                 getEnv("SQLFLOW_TEST_DB_MYSQL_USER", "root"),
			Passwd:               getEnv("SQLFLOW_TEST_DB_MYSQL_PASSWD", "root"),
			Net:                  getEnv("SQLFLOW_TEST_DB_MYSQL_NET", "tcp"),
			Addr:                 getEnv("SQLFLOW_TEST_DB_MYSQL_ADDR", "127.0.0.1:3306"),
			AllowNativePasswords: true,
		}
		testDB, e = Open(fmt.Sprintf("mysql://%s", cfg.FormatDSN()))
		assertNoErr(e)
		_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS iris;")
		assertNoErr(e)
		_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS churn;")
		assertNoErr(e)
		_, e = testDB.Exec("CREATE DATABASE IF NOT EXISTS sqlflow_models;")
		assertNoErr(e)
		defer testDB.Close()
	case "hive":
		// NOTE: sample dataset is written in
		// https://github.com/sql-machine-learning/gohive/blob/develop/docker/entrypoint.sh#L123
		os.Exit(0)
	default:
		e = fmt.Errorf("unrecognized environment variable SQLFLOW_TEST_DB %s", dbms)
	}
	assertNoErr(e)

	assertNoErr(Popularize(testDB, "testdata/iris.sql"))
	assertNoErr(Popularize(testDB, "testdata/churn.sql"))

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
