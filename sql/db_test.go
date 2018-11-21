package sql

import (
	"database/sql"
	"log"
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
		log.Panicf("verify cannot connect to MySQL: %q", e)
	}
	testDB = db

	defer testDB.Close()
	os.Exit(m.Run())
}
