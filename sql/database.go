package sql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// Database holds configuration like  mysql, sqlite...
type Database struct {
	DriverName string
	User       string
	Password   string
	Addr       string
	DataSource string
	Conn       *sql.DB
}

func (db *Database) Open() error {
	driver := strings.ToUpper(db.DriverName)
	var err error
	switch driver {
	case "MYSQL":
		db.Conn, err = db.openMysql()
	case "SQLITE3":
		db.Conn, err = db.openSQLite3()
	default:
		db.Conn, err = nil, fmt.Errorf("Not implemented yet")
	}
	return err
}

func (db *Database) Close() {
	if db.Conn != nil {
		db.Conn.Close()
	}
	db.Conn = nil
}

func (db *Database) openMysql() (*sql.DB, error) {
	myCfg := &mysql.Config{
		User:   db.User,
		Passwd: db.Password,
		Addr:   db.Addr,
	}
	return sql.Open("mysql", myCfg.FormatDSN())
}

func (db *Database) openSQLite3() (*sql.DB, error) {
	return sql.Open("sqlite3", db.DataSource)
}
