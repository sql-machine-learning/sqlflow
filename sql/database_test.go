package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseOpenMysql(t *testing.T) {
	a := assert.New(t)
	db := &Database{
		User:       "root",
		Password:   "root",
		Addr:       "localhost:3306",
		DriverName: "mysql",
	}
	e := db.Open()
	a.NoError(e)
	defer db.Close()

	_, e = db.Conn.Exec("show databases")
	a.NoError(e)
}

func TestDatabaseOpenSQLite3(t *testing.T) {
	a := assert.New(t)
	db := &Database{
		DataSource: "test",
		DriverName: "sqlite3",
	}
	e := db.Open()
	a.NoError(e)
	defer db.Close()
	// TODO: need more tests
}
