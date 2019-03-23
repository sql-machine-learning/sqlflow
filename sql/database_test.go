package sql

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseOpenMysql(t *testing.T) {
	a := assert.New(t)
	cfg := &mysql.Config{
		User:                 "root",
		Passwd:               "root",
		Net:                  "tcp",
		Addr:                 "localhost:3306",
		AllowNativePasswords: true,
	}
	db, e := Open("mysql", cfg.FormatDSN())
	a.NoError(e)
	defer db.Close()

	_, e = db.Exec("show databases")
	a.NoError(e)
}

func TestDatabaseOpenSQLite3(t *testing.T) {
	a := assert.New(t)
	db, e := Open("sqlite3", "test")
	a.NoError(e)
	defer db.Close()
	// TODO: need more tests
}
