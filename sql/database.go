package sql

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// DB extends sql.DB
type DB struct {
	driverName     string
	dataSourceName string
	*sql.DB
}

// Open opens a database specified by its database driver name and a
// driver-specific data source name, usually consisting of at least a
// database name and connection information.
//
// In addition to sql.Open, it also does the book keeping on driverName and
// dataSourceName
func Open(driverName, dataSourceName string) (*DB, error) {
	db := &DB{driverName: driverName, dataSourceName: dataSourceName}

	var err error
	switch driverName {
	case "sqlite3", "mysql":
		db.DB, err = sql.Open(driverName, dataSourceName)
	default:
		db.DB, err = nil, fmt.Errorf("sqlfow currently doesn't support DB %v", driverName)
	}

	return db, err
}
